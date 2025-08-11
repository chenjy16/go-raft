package raft

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
	
	"github.com/chenjianyu/go-raft/raft/logging"
)

// Node 表示 Raft 节点
type Node struct {
	mu sync.RWMutex

	// 节点配置
	config *Config

	// 当前状态
	id    uint64
	state NodeState

	// 持久化状态
	currentTerm uint64
	votedFor    uint64
	leader      uint64  // 添加 leader 字段来追踪当前 Leader

	// 日志相关
	commitIndex uint64
	lastApplied uint64

	// Leader 状态
	nextIndex  map[uint64]uint64
	matchIndex map[uint64]uint64

	// 日志
	raftLog *RaftLog

	// 集群成员
	peers map[uint64]bool

	// 选举相关
	electionElapsed           int
	heartbeatElapsed          int
	randomizedElectionTimeout int
	votes                     map[uint64]bool  // 添加投票计数字段

	// 消息处理
	msgs []Message

	// 状态机
	stateMachine StateMachine

	// ReadIndex 管理器
	readIndexManager *ReadIndexManager

	// Learner 管理器
	learnerManager *LearnerManager

	// PreVote 管理器
	preVoteManager *PreVoteManager

	// 网络分区检测器
	partitionDetector *NetworkPartitionDetector

	// 事件发射器
	eventEmitter logging.EventEmitter

	// 通道
	readyc     chan Ready
	advancec   chan struct{}
	tickc      chan struct{}
	recvc      chan Message
	confc      chan ConfChange
	confstatec chan ConfState
	stopc      chan struct{}
	done       chan struct{}
}

// StateMachine 状态机接口
type StateMachine interface {
	Apply(data []byte) ([]byte, error)
	Snapshot() ([]byte, error)
	Restore(data []byte) error
}

// ConfChange 配置变更
type ConfChange struct {
	Type    ConfChangeType `json:"type"`
	NodeID  uint64         `json:"node_id"`
	Context []byte         `json:"context,omitempty"`
}

// ConfChangeType 配置变更类型
type ConfChangeType int

const (
	ConfChangeAddNode ConfChangeType = iota
	ConfChangeRemoveNode
)

// ConfState 配置状态
type ConfState struct {
	Nodes []uint64 `json:"nodes"`
}

// NewNode 创建新的 Raft 节点
func NewNode(config *Config, stateMachine StateMachine) *Node {
	if config == nil {
		config = DefaultConfig()
	}

	raftLog := NewRaftLog(config.Storage)

	n := &Node{
		config:           config,
		id:               config.ID,
		state:            StateFollower,
		raftLog:          raftLog,
		stateMachine:     stateMachine,
		peers:            make(map[uint64]bool),
		nextIndex:        make(map[uint64]uint64),
		matchIndex:       make(map[uint64]uint64),
		votes:            make(map[uint64]bool),  // 初始化投票计数
		readIndexManager:  NewReadIndexManager(config.ReadOnlyOption, config.LeaseTimeout),
		learnerManager:    NewLearnerManager(),
		preVoteManager:    NewPreVoteManager(config.PreVote, config.CheckQuorum),
		partitionDetector: NewNetworkPartitionDetector(true, config.ElectionTimeout*3),
		readyc:            make(chan Ready, 1),
		advancec:         make(chan struct{}),
		tickc:            make(chan struct{}),
		recvc:            make(chan Message, 256),
		confc:            make(chan ConfChange),
		confstatec:       make(chan ConfState),
		stopc:            make(chan struct{}),
		done:             make(chan struct{}),
	}

	// 初始化集群成员（只包含自己）
	n.peers[n.id] = true

	// 从存储恢复状态
	hardState, err := config.Storage.InitialState()
	if err != nil {
		panic(err)
	}

	if !IsEmptyHardState(hardState) {
		n.currentTerm = hardState.Term
		n.votedFor = hardState.Vote
		n.commitIndex = hardState.Commit
	}

	n.resetRandomizedElectionTimeout()

	return n
}

// SetEventEmitter 设置事件发射器
func (n *Node) SetEventEmitter(emitter logging.EventEmitter) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.eventEmitter = emitter
}

// emitEvent 发射事件的便利方法
func (n *Node) emitEvent(event logging.Event) {
	if n.eventEmitter != nil {
		n.eventEmitter.EmitEvent(event)
	}
}

// Start 启动节点
func (n *Node) Start() error {
	// 发射节点启动事件
	if n.eventEmitter != nil {
		event := &logging.NodeStartingEvent{
			BaseEvent: logging.NewBaseEvent(logging.EventNodeStarting, n.id),
			Config: map[string]interface{}{
				"node_id": n.id,
				"state":   n.state.String(),
			},
		}
		n.eventEmitter.EmitEvent(event)
	}
	
	go n.run()
	
	// 发射节点启动完成事件
	if n.eventEmitter != nil {
		event := &logging.NodeStartedEvent{
			BaseEvent: logging.NewBaseEvent(logging.EventNodeStarted, n.id),
			Duration:  0,
			Error:     nil,
		}
		n.eventEmitter.EmitEvent(event)
	}
	
	return nil
}

// Stop 停止节点
func (n *Node) Stop() {
	// 发射节点停止事件
	if n.eventEmitter != nil {
		event := &logging.BaseEvent{
			Type:      logging.EventNodeStopping,
			Timestamp: time.Now(),
			NodeID:    n.id,
		}
		n.eventEmitter.EmitEvent(event)
	}
	
	n.mu.Lock()
	defer n.mu.Unlock()
	
	select {
	case n.stopc <- struct{}{}:
	case <-n.done:
		// 发射节点已停止事件
		if n.eventEmitter != nil {
			event := &logging.BaseEvent{
				Type:      logging.EventNodeStopped,
				Timestamp: time.Now(),
				NodeID:    n.id,
			}
			n.eventEmitter.EmitEvent(event)
		}
		return // already stopped
	default:
		return // already stopping
	}
	<-n.done
	
	// 发射节点已停止事件
	if n.eventEmitter != nil {
		event := &logging.BaseEvent{
			Type:      logging.EventNodeStopped,
			Timestamp: time.Now(),
			NodeID:    n.id,
		}
		n.eventEmitter.EmitEvent(event)
	}
}

// Ready 返回 Ready 通道
func (n *Node) Ready() <-chan Ready {
	return n.readyc
}

// Advance 通知节点处理完 Ready
func (n *Node) Advance() {
	select {
	case n.advancec <- struct{}{}:
	case <-n.done:
	}
}

// SetPeers 设置集群成员
func (n *Node) SetPeers(peerIDs []uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 清空现有 peers
	n.peers = make(map[uint64]bool)
	
	// 添加所有 peers（包括自己）
	for _, id := range peerIDs {
		n.peers[id] = true
	}
	
	// 确保自己在 peers 中
	n.peers[n.id] = true
}

// Step 处理消息
func (n *Node) Step(ctx context.Context, msg Message) error {
	// 快速检查是否已停止（非阻塞）
	select {
	case <-n.done:
		return fmt.Errorf("node stopped")
	default:
	}
	
	// 特别处理 MsgPropose：如果不是 Leader，立即拒绝
	if msg.Type == MsgPropose {
		n.mu.RLock()
		isLeader := n.state == StateLeader
		n.mu.RUnlock()
		
		if !isLeader {
			return ErrNotLeader
		}
	}
	
	select {
	case n.recvc <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-n.done:
		return fmt.Errorf("node stopped")
	}
}

// Propose 提议新的日志条目
func (n *Node) Propose(ctx context.Context, data []byte) error {
	// 拒绝已停止节点
	select {
	case <-n.done:
		return fmt.Errorf("node stopped")
	default:
	}
	
	// 只有 Leader 才能处理提议
	n.mu.RLock()
	isLeader := n.state == StateLeader
	n.mu.RUnlock()
	if !isLeader {
		return ErrNotLeader
	}
	
	// 同步处理本地提议，避免依赖 run 循环
	msg := Message{Type: MsgPropose, Data: data}
	n.step(msg)
	return nil
}

// duplicate Step removed

// ProposeConfChange 提议配置变更
func (n *Node) ProposeConfChange(ctx context.Context, cc ConfChange) error {
	data := encodeConfChange(cc)
	return n.Step(ctx, Message{
		Type: MsgPropose,
		Data: data,
	})
}

// ApplyConfChange 应用配置变更
func (n *Node) ApplyConfChange(cc ConfChange) *ConfState {
	n.mu.Lock()
	defer n.mu.Unlock()

	switch cc.Type {
	case ConfChangeAddNode:
		n.peers[cc.NodeID] = true
		if n.state == StateLeader {
			n.nextIndex[cc.NodeID] = n.raftLog.LastIndex() + 1
			n.matchIndex[cc.NodeID] = 0
		}
	case ConfChangeRemoveNode:
		delete(n.peers, cc.NodeID)
		if n.state == StateLeader {
			delete(n.nextIndex, cc.NodeID)
			delete(n.matchIndex, cc.NodeID)
		}
	}

	nodes := make([]uint64, 0, len(n.peers))
	for id := range n.peers {
		nodes = append(nodes, id)
	}

	return &ConfState{Nodes: nodes}
}

// Tick 处理时钟滴答
func (n *Node) Tick() {
	select {
	case n.tickc <- struct{}{}:
	case <-n.done:
	}
}

// 主运行循环
func (n *Node) run() {
	defer close(n.done)

	var ready Ready

	for {
		if n.hasReady() {
			ready = n.newReady()
			select {
			case n.readyc <- ready:
			case <-n.stopc:
				return
			}

			select {
			case <-n.advancec:
				n.advance(ready)
			case <-n.stopc:
				return
			}
		} else {
			select {
			case <-n.tickc:
				n.tick()
			case msg := <-n.recvc:
				n.step(msg)
			case <-n.stopc:
				return
			}
		}
	}
}

// 检查是否有准备好的状态
func (n *Node) hasReady() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return len(n.msgs) > 0 ||
		n.raftLog.HasNextEnts() ||
		len(n.raftLog.UnstableEntries()) > 0
}

// 创建 Ready 状态
func (n *Node) newReady() Ready {
	n.mu.Lock()
	defer n.mu.Unlock()

	ready := Ready{
		HardState: HardState{
			Term:   n.currentTerm,
			Vote:   n.votedFor,
			Commit: n.commitIndex,
		},
		Messages: append([]Message(nil), n.msgs...),
	}

	// 填充 SoftState 以便外部获取 leader 和状态
	ready.SoftState = &SoftState{Lead: n.leader, State: n.state}

	hasNextEnts := n.raftLog.HasNextEnts()
	
	if hasNextEnts {
		ready.CommittedEntries = n.raftLog.NextEnts()
		// 发射日志提交事件
		if n.eventEmitter != nil {
			event := &logging.LogCommittedEvent{
				BaseEvent:   logging.NewBaseEvent(logging.EventLogCommitted, n.id),
				CommitIndex: n.raftLog.committed,
				EntryCount:  len(ready.CommittedEntries),
				Duration:    0, // 这里可以根据需要计算实际持续时间
			}
			n.eventEmitter.EmitEvent(event)
		}
	}

	if unstable := n.raftLog.UnstableEntries(); len(unstable) > 0 {
		ready.Entries = unstable
	}

	n.msgs = n.msgs[:0]

	return ready
}

// 处理 Ready 完成后的状态更新
func (n *Node) advance(ready Ready) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 更新提交索引
	if !IsEmptyHardState(ready.HardState) && ready.HardState.Commit > n.raftLog.committed {
		n.raftLog.CommitTo(ready.HardState.Commit)
		n.commitIndex = ready.HardState.Commit
	}

	if len(ready.Entries) > 0 {
		lastIndex := ready.Entries[len(ready.Entries)-1].Index
		n.raftLog.StableTo(lastIndex)
	}

	if len(ready.CommittedEntries) > 0 {
		lastIndex := ready.CommittedEntries[len(ready.CommittedEntries)-1].Index
		n.raftLog.AppliedTo(lastIndex)
		n.lastApplied = lastIndex
	}
}

// 时钟滴答处理
func (n *Node) tick() {
    n.mu.Lock()
    switch n.state {
    case StateFollower, StateCandidate:
        n.electionElapsed++
        // 使用随机化的选举超时时间以避免选举活锁
        if n.electionElapsed >= n.randomizedElectionTimeout {
            n.electionElapsed = 0
            // 直接调用相应的选举函数，不需要通过消息
            if n.config.PreVote {
                n.campaignPreVote()
            } else {
                n.campaign()
            }
        }
    case StateLeader:
        n.heartbeatElapsed++
        // 领导者也需要推进 electionElapsed 以便周期性进行 quorum 检查
        n.electionElapsed++
        if n.heartbeatElapsed >= n.config.HeartbeatTick {
            n.heartbeatElapsed = 0
            n.bcastHeartbeat()
        }
        // Leader 定期检查 quorum
        if n.config.CheckQuorum && n.electionElapsed >= n.config.ElectionTick {
            n.checkQuorum()
            n.electionElapsed = 0
        }
    }
    n.mu.Unlock()
}

// 消息处理
func (n *Node) step(msg Message) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// PreVote/PreVoteResp 不应影响当前任期，也不应因任期不匹配被丢弃
	if msg.Type == MsgPreVote || msg.Type == MsgPreVoteResp {
		n.stepRemote(msg)
		return
	}

	switch {
	case msg.Term == 0:
		// 本地消息
		n.stepLocal(msg)
	case msg.Term > n.currentTerm:
		// 更高的任期，转为 follower
		n.becomeFollower(msg.Term, 0)
		n.stepRemote(msg)
	case msg.Term < n.currentTerm:
		// 较低的任期，忽略
		return
	default:
		// 相同任期
		n.stepRemote(msg)
	}
}

// 处理本地消息
func (n *Node) stepLocal(msg Message) {
	switch msg.Type {
	case MsgRequestVote:
		if n.config.PreVote {
			// 使用预投票来避免不必要的term提升
			n.campaignPreVote()
		} else {
			n.campaign()
		}
	case MsgHeartbeat:
		n.bcastHeartbeat()
	case MsgPropose:
		if n.state != StateLeader {
			return
		}
		n.appendEntry(msg.Data)
	}
}

// 处理远程消息
func (n *Node) stepRemote(msg Message) {
	switch msg.Type {
	case MsgRequestVote:
		n.handleRequestVote(msg)
	case MsgRequestVoteResp:
		n.handleRequestVoteResp(msg)
	case MsgAppendEntries:
		n.handleAppendEntries(msg)
	case MsgAppendEntriesResp:
		n.handleAppendEntriesResp(msg)
	case MsgHeartbeat:
		n.handleHeartbeat(msg)
	case MsgHeartbeatResp:
		n.handleHeartbeatResp(msg)
	case MsgReadIndex:
		n.handleReadIndex(msg)
	case MsgReadIndexResp:
		n.handleReadIndexResp(msg)
	case MsgPreVote:
		n.handlePreVote(msg)
	case MsgPreVoteResp:
		n.handlePreVoteResp(msg)
	case MsgTimeoutNow:
		n.handleTimeoutNow(msg)
	case MsgCheckQuorum:
		n.handleCheckQuorum(msg)
	}
}

// 发起选举
func (n *Node) campaign() {
	n.becomeCandidate()

	// 向所有节点发送投票请求
	for id := range n.peers {
		if id == n.id {
			continue
		}

		n.send(Message{
			Type:     MsgRequestVote,
			From:     n.id,
			To:       id,
			Term:     n.currentTerm,
			LogIndex: n.raftLog.LastIndex(),
			LogTerm:  n.raftLog.LastTerm(),
		})
	}
}

// 转为 follower
func (n *Node) becomeFollower(term, lead uint64) {
	n.state = StateFollower
	n.currentTerm = term
	n.votedFor = 0
	n.leader = lead  // 设置 leader 字段
	n.electionElapsed = 0
	n.resetRandomizedElectionTimeout()
}

// 转为 candidate
func (n *Node) becomeCandidate() {
	n.state = StateCandidate
	n.currentTerm++
	n.votedFor = n.id
	n.leader = 0  // 候选状态下没有 leader
	n.electionElapsed = 0
	n.resetRandomizedElectionTimeout()
	
	// 重置投票计数，给自己投票
	n.votes = make(map[uint64]bool)
	n.votes[n.id] = true
}

// 转为 leader
func (n *Node) becomeLeader() {
	n.state = StateLeader
	n.leader = n.id  // 自己成为 leader
	n.heartbeatElapsed = 0
	
	// 确保 currentTerm 至少为 1
	if n.currentTerm == 0 {
		n.currentTerm = 1
	}

	// 初始化 nextIndex 和 matchIndex
	lastIndex := n.raftLog.LastIndex()
	for id := range n.peers {
		if id == n.id {
			continue
		}
		n.nextIndex[id] = lastIndex + 1
		n.matchIndex[id] = 0
	}

	// 添加一个空日志条目以触发 Ready 状态并确保状态变化被持久化
	// 这样外部可以及时感知到 Leader 状态变化
	n.appendEntry(nil)

	// 发送心跳
	n.bcastHeartbeat()
}

// 广播心跳
func (n *Node) bcastHeartbeat() {
	for id := range n.peers {
		if id == n.id {
			continue
		}
		n.sendHeartbeat(id)
	}
}

// 发送心跳
func (n *Node) sendHeartbeat(to uint64) {
	n.send(Message{
		Type:        MsgHeartbeat,
		From:        n.id,
		To:          to,
		Term:        n.currentTerm,
		CommitIndex: n.commitIndex,
	})
}

// 追加日志条目
func (n *Node) appendEntry(data []byte) {
	entry := LogEntry{
		Index: n.raftLog.LastIndex() + 1,
		Term:  n.currentTerm,
		Data:  data,
	}

	n.raftLog.Append(entry)
	n.matchIndex[n.id] = entry.Index

	// 复制到其他节点
	n.bcastAppend()
}

// 广播日志条目
func (n *Node) bcastAppend() {
	for id := range n.peers {
		if id == n.id {
			continue
		}
		n.sendAppend(id)
	}
}

// 发送日志条目
func (n *Node) sendAppend(to uint64) {
	prevIndex := n.nextIndex[to] - 1
	prevTerm, err := n.raftLog.Term(prevIndex)
	if err != nil {
		// 发送快照
		return
	}

	entries, err := n.raftLog.Entries(n.nextIndex[to], n.raftLog.LastIndex()+1)
	if err != nil {
		return
	}

	n.send(Message{
		Type:        MsgAppendEntries,
		From:        n.id,
		To:          to,
		Term:        n.currentTerm,
		LogIndex:    prevIndex,
		LogTerm:     prevTerm,
		Entries:     entries,
		CommitIndex: n.commitIndex,
	})
}

// 发送消息
func (n *Node) send(msg Message) {
	n.msgs = append(n.msgs, msg)
}

// 处理投票请求
func (n *Node) handleRequestVote(msg Message) {
	canVote := n.votedFor == 0 || n.votedFor == msg.From
	upToDate := n.raftLog.IsUpToDate(msg.LogIndex, msg.LogTerm)

	if canVote && upToDate {
		n.votedFor = msg.From
		n.electionElapsed = 0
		n.resetRandomizedElectionTimeout()

		n.send(Message{
			Type:        MsgRequestVoteResp,
			From:        n.id,
			To:          msg.From,
			Term:        n.currentTerm,
			VoteGranted: true,
		})
	} else {
		n.send(Message{
			Type:        MsgRequestVoteResp,
			From:        n.id,
			To:          msg.From,
			Term:        n.currentTerm,
			VoteGranted: false,
		})
	}
}

// 处理投票响应
func (n *Node) handleRequestVoteResp(msg Message) {
	if n.state != StateCandidate {
		return
	}

	if msg.VoteGranted {
		// 记录投票
		n.votes[msg.From] = true
		
		// 计算获得的票数
		voteCount := len(n.votes)
		
		// 检查是否获得大多数票
		if voteCount >= n.quorum() {
			n.becomeLeader()
		}
	}
}

// 处理日志追加请求
func (n *Node) handleAppendEntries(msg Message) {
	n.electionElapsed = 0
	n.resetRandomizedElectionTimeout()

	// 更新 leader 信息
	n.leader = msg.From

	if msg.LogIndex > 0 && !n.raftLog.MatchTerm(msg.LogIndex, msg.LogTerm) {
		n.send(Message{
			Type:    MsgAppendEntriesResp,
			From:    n.id,
			To:      msg.From,
			Term:    n.currentTerm,
			Success: false,
		})
		return
	}

	if len(msg.Entries) > 0 {
		n.raftLog.Append(msg.Entries...)
	}

	if msg.CommitIndex > n.commitIndex {
		n.commitIndex = min(msg.CommitIndex, n.raftLog.LastIndex())
	}

	n.send(Message{
		Type:     MsgAppendEntriesResp,
		From:     n.id,
		To:       msg.From,
		Term:     n.currentTerm,
		Success:  true,
		LogIndex: n.raftLog.LastIndex(), // 设置当前日志的最后索引
	})
}

// 处理日志追加响应
func (n *Node) handleAppendEntriesResp(msg Message) {
    if n.state != StateLeader {
        return
    }

    // 收到响应即认为与该节点有联系，更新活跃状态
    if n.preVoteManager != nil {
        n.preVoteManager.UpdateHeartbeat(msg.From)
    }
    if n.partitionDetector != nil {
        n.partitionDetector.UpdateContact(msg.From)
    }

    if msg.Success {
        // 更新跟随者的matchIndex和nextIndex
        n.matchIndex[msg.From] = msg.LogIndex
        n.nextIndex[msg.From] = msg.LogIndex + 1
        n.maybeCommit()
    } else {
        // 回退 nextIndex 重试
        if msg.LogIndex > 0 {
            n.nextIndex[msg.From] = msg.LogIndex
        } else if n.nextIndex[msg.From] > 1 {
            n.nextIndex[msg.From]--
        }
        n.sendAppend(msg.From)
    }
}

// 处理心跳
func (n *Node) handleHeartbeat(msg Message) {
    n.electionElapsed = 0
    n.resetRandomizedElectionTimeout()

    // 更新 leader 信息
    n.leader = msg.From

    // 更新预投票与分区检测的心跳/接触时间
    if n.preVoteManager != nil {
        n.preVoteManager.UpdateHeartbeat(msg.From)
    }
    if n.partitionDetector != nil {
        n.partitionDetector.UpdateContact(msg.From)
    }

    n.send(Message{
        Type: MsgHeartbeatResp,
        From: n.id,
        To:   msg.From,
        Term: n.currentTerm,
    })
}

// 处理心跳响应
func (n *Node) handleHeartbeatResp(msg Message) {
    // Leader更新对应节点的活跃信息
    if n.state == StateLeader {
        if n.preVoteManager != nil {
            n.preVoteManager.UpdateHeartbeat(msg.From)
        }
        if n.partitionDetector != nil {
            n.partitionDetector.UpdateContact(msg.From)
        }
    }
}

// 尝试提交日志
func (n *Node) maybeCommit() {
	// 收集所有节点的 matchIndex，包括 Leader 自己
	indices := make([]uint64, 0, len(n.peers))
	
	// 添加 Leader 自己的 matchIndex
	indices = append(indices, n.raftLog.LastIndex())
	
	// 添加其他节点的 matchIndex
	for id, index := range n.matchIndex {
		if id != n.id {
			indices = append(indices, index)
		}
	}

	// 从高到低检查每个索引是否能被大多数节点确认
	for index := n.raftLog.LastIndex(); index > n.commitIndex; index-- {
		count := 0
		// 计算有多少节点的 matchIndex >= index
		for _, mi := range indices {
			if mi >= index {
				count++
			}
		}
		
		// 如果达到法定人数且是当前任期的条目
		if count >= n.quorum() {
			term, err := n.raftLog.Term(index)
			if err == nil && term == n.currentTerm {
				n.commitIndex = index
				n.raftLog.CommitTo(index)
				break
			}
		}
	}
}

// 计算法定人数
func (n *Node) quorum() int {
	// 集群大小就是 peers 的数量（包含自己）
	clusterSize := len(n.peers)
	return clusterSize/2 + 1
}

// 心跳超时
func (n *Node) heartbeatTimeout() int {
	return n.config.HeartbeatTick
}

// 重置随机选举超时
func (n *Node) resetRandomizedElectionTimeout() {
	base := n.config.ElectionTick
	if base <= 0 {
		base = 10 // 默认值
	}
	n.randomizedElectionTimeout = base + rand.Intn(base)
}

// 编码配置变更
func encodeConfChange(cc ConfChange) []byte {
	// 简化实现，实际应该使用 protobuf 或 json
	return []byte(fmt.Sprintf("%d:%d", cc.Type, cc.NodeID))
}

// 处理ReadIndex请求
func (n *Node) handleReadIndex(msg Message) {
	if n.state != StateLeader {
		// 非leader节点，拒绝ReadIndex请求
		n.send(Message{
			Type:    MsgReadIndexResp,
			From:    n.id,
			To:      msg.From,
			Term:    n.currentTerm,
			Success: false,
		})
		return
	}

	// 添加到ReadIndex管理器
	n.readIndexManager.AddRequest(msg)

	// 发送心跳确认leader身份
	n.bcastHeartbeat()

	// 响应ReadIndex请求
	n.send(Message{
		Type:      MsgReadIndexResp,
		From:      n.id,
		To:        msg.From,
		Term:      n.currentTerm,
		Success:   true,
		ReadIndex: n.commitIndex,
		Context:   msg.Context,
	})
}

// 处理ReadIndex响应
func (n *Node) handleReadIndexResp(msg Message) {
	if msg.Success {
		n.readIndexManager.RecvAck(msg.From, msg.Context)
	}
}

// 处理CheckQuorum消息
func (n *Node) handleCheckQuorum(msg Message) {
	// 更新心跳时间
	if n.preVoteManager != nil {
		n.preVoteManager.UpdateHeartbeat(msg.From)
	}
	if n.partitionDetector != nil {
		n.partitionDetector.UpdateContact(msg.From)
	}

	// 响应CheckQuorum
	n.send(Message{
		Type: MsgHeartbeatResp,
		From: n.id,
		To:   msg.From,
		Term: n.currentTerm,
	})
}
