package raft

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PreVoteManager 管理PreVote功能
type PreVoteManager struct {
	mu              sync.RWMutex
	enabled         bool
	preVoteGranted  map[uint64]bool
	preVoteTerm     uint64
	checkQuorum     bool
	lastHeartbeat   map[uint64]time.Time
	quorumActive    bool
	lastQuorumCheck time.Time
}

// NewPreVoteManager 创建新的PreVote管理器
func NewPreVoteManager(enabled, checkQuorum bool) *PreVoteManager {
	return &PreVoteManager{
		enabled:         enabled,
		checkQuorum:     checkQuorum,
		preVoteGranted:  make(map[uint64]bool),
		lastHeartbeat:   make(map[uint64]time.Time),
		quorumActive:    true,
		lastQuorumCheck: time.Now(),
	}
}

// StartPreVote 开始PreVote流程
func (pvm *PreVoteManager) StartPreVote(term uint64) {
	pvm.mu.Lock()
	defer pvm.mu.Unlock()

	pvm.preVoteTerm = term
	pvm.preVoteGranted = make(map[uint64]bool)
}

// GrantPreVote 授予PreVote
func (pvm *PreVoteManager) GrantPreVote(from uint64) {
	pvm.mu.Lock()
	defer pvm.mu.Unlock()

	pvm.preVoteGranted[from] = true
}

// HasPreVoteQuorum 检查是否有PreVote法定人数
func (pvm *PreVoteManager) HasPreVoteQuorum(quorum int) bool {
	pvm.mu.RLock()
	defer pvm.mu.RUnlock()

	granted := 1 // 包括自己
	for _, vote := range pvm.preVoteGranted {
		if vote {
			granted++
		}
	}

	return granted >= quorum
}

// UpdateHeartbeat 更新心跳时间
func (pvm *PreVoteManager) UpdateHeartbeat(from uint64) {
	pvm.mu.Lock()
	defer pvm.mu.Unlock()

	pvm.lastHeartbeat[from] = time.Now()
}

// HasRecentHeartbeat 判断来自指定节点的心跳是否在超时时间内
func (pvm *PreVoteManager) HasRecentHeartbeat(from uint64, timeout time.Duration) bool {
	pvm.mu.RLock()
	t, ok := pvm.lastHeartbeat[from]
	pvm.mu.RUnlock()
	if !ok {
		return false
	}
	return time.Since(t) < timeout
}

// CheckQuorumActive 检查法定人数是否活跃
func (pvm *PreVoteManager) CheckQuorumActive(quorum int, timeout time.Duration) bool {
	if !pvm.checkQuorum {
		return true
	}

	pvm.mu.Lock()
	defer pvm.mu.Unlock()

	now := time.Now()
	if now.Sub(pvm.lastQuorumCheck) < timeout/2 {
		return pvm.quorumActive
	}

	activeNodes := 1 // 包括自己
	for _, lastSeen := range pvm.lastHeartbeat {
		if now.Sub(lastSeen) < timeout {
			activeNodes++
		}
	}

	pvm.quorumActive = activeNodes >= quorum
	pvm.lastQuorumCheck = now
	return pvm.quorumActive
}

// IsEnabled 检查PreVote是否启用
func (pvm *PreVoteManager) IsEnabled() bool {
	pvm.mu.RLock()
	defer pvm.mu.RUnlock()

	return pvm.enabled
}

// 扩展Node的方法以支持PreVote

// campaignPreVote 开始PreVote竞选（调用方需已持有 n.mu 锁）
func (n *Node) campaignPreVote() {
	if !n.preVoteManager.IsEnabled() {
		// 如果未启用PreVote，直接进行正常选举
		n.campaign()
		return
	}

	// 开始PreVote（确保使用至少为1的term），调用方已加锁
	term := n.currentTerm + 1
	if term == 0 { // 溢出保护，理论上不会发生
		term = 1
	}
	n.preVoteManager.StartPreVote(term)

	// 向所有节点发送PreVote请求
	for id := range n.peers {
		if id != n.id {
			n.sendPreVote(id)
		}
	}
}

// sendPreVote 发送PreVote请求
func (n *Node) sendPreVote(to uint64) {
	lastLogIndex := n.raftLog.LastIndex()
	lastLogTerm := n.raftLog.LastTerm()

	msg := Message{
		Type:     MsgPreVote,
		From:     n.id,
		To:       to,
		Term:     n.currentTerm + 1, // PreVote使用下一个term
		LogIndex: lastLogIndex,
		LogTerm:  lastLogTerm,
		PreVote:  true,
	}

	n.send(msg)
}

// handlePreVote 处理PreVote请求
func (n *Node) handlePreVote(msg Message) {
	// 检查是否可以授予PreVote
	canGrant := n.canGrantPreVote(msg)

	resp := Message{
		Type:        MsgPreVoteResp,
		From:        n.id,
		To:          msg.From,
		Term:        msg.Term,
		VoteGranted: canGrant,
	}

	n.send(resp)
}

// canGrantPreVote 检查是否可以授予PreVote
func (n *Node) canGrantPreVote(msg Message) bool {
	// PreVote的条件：
	// 1. 候选者的日志至少和自己一样新
	// 2. 没有收到来自当前leader的最近心跳（如果有leader的话）

	lastLogIndex := n.raftLog.LastIndex()
	lastLogTerm := n.raftLog.LastTerm()

	// 检查日志是否至少一样新
	if msg.LogTerm < lastLogTerm {
		return false
	}
	if msg.LogTerm == lastLogTerm && msg.LogIndex < lastLogIndex {
		return false
	}

	// 如果集群刚初始化（currentTerm <= 1 且 leader == 0），允许PreVote
	// 这包括了节点启动后的初始选举阶段
	if n.currentTerm <= 1 && n.leader == 0 {
		return true
	}

	// 如果我们是Follower且知道有leader，检查是否最近收到了心跳
	if n.state == StateFollower && n.leader != 0 {
		// 使用配置的选举超时时间
		electionTimeout := n.config.ElectionTimeout
		if electionTimeout == 0 {
			// 如果没有配置 ElectionTimeout，使用 tick 计算
			electionTimeout = time.Duration(n.config.ElectionTick * 100) * time.Millisecond
		}
		if n.preVoteManager.HasRecentHeartbeat(n.leader, electionTimeout) {
			// 最近收到leader心跳，拒绝PreVote
			return false
		}
	}

	return true
}

// handlePreVoteResp 处理PreVote响应
func (n *Node) handlePreVoteResp(msg Message) {
	if msg.VoteGranted {
		n.preVoteManager.GrantPreVote(msg.From)

		// 检查是否获得了足够的PreVote，并确保仍为Follower且Term匹配
		if n.preVoteManager.HasPreVoteQuorum(n.quorum()) && n.state == StateFollower && msg.Term == n.currentTerm+1 {
			// 获得了PreVote法定人数，开始正式选举
			n.campaign()
		}
	}
}

// checkQuorum 检查法定人数
func (n *Node) checkQuorum() {
	if !n.config.CheckQuorum || n.state != StateLeader {
		return
	}

	if !n.preVoteManager.CheckQuorumActive(n.quorum(), n.config.ElectionTimeout) {
		// 法定人数不活跃，退回到follower
		n.becomeFollower(n.currentTerm, 0)
	}
}

// NetworkPartitionDetector 网络分区检测器
type NetworkPartitionDetector struct {
	mu                sync.RWMutex
	enabled           bool
	partitionTimeout  time.Duration
	lastContact       map[uint64]time.Time
	suspectedNodes    map[uint64]bool
	partitionCallback func([]uint64) // 分区检测回调
}

// NewNetworkPartitionDetector 创建网络分区检测器
func NewNetworkPartitionDetector(enabled bool, timeout time.Duration) *NetworkPartitionDetector {
	return &NetworkPartitionDetector{
		enabled:          enabled,
		partitionTimeout: timeout,
		lastContact:      make(map[uint64]time.Time),
		suspectedNodes:   make(map[uint64]bool),
	}
}

// UpdateContact 更新节点联系时间
func (npd *NetworkPartitionDetector) UpdateContact(nodeID uint64) {
	if !npd.enabled {
		return
	}

	npd.mu.Lock()
	defer npd.mu.Unlock()

	npd.lastContact[nodeID] = time.Now()
	delete(npd.suspectedNodes, nodeID) // 移除怀疑状态
}

// DetectPartition 检测网络分区
func (npd *NetworkPartitionDetector) DetectPartition() []uint64 {
	if !npd.enabled {
		return nil
	}

	npd.mu.Lock()
	defer npd.mu.Unlock()

	now := time.Now()
	var suspectedNodes []uint64
	for id, last := range npd.lastContact {
		if now.Sub(last) > npd.partitionTimeout {
			npd.suspectedNodes[id] = true
			suspectedNodes = append(suspectedNodes, id)
		}
	}

	return suspectedNodes
}

// SetPartitionCallback 设置分区检测回调
func (npd *NetworkPartitionDetector) SetPartitionCallback(callback func([]uint64)) {
	npd.mu.Lock()
	defer npd.mu.Unlock()

	npd.partitionCallback = callback
}

// StartPartitionDetection 启动分区检测（示例）
func (n *Node) StartPartitionDetection() {
	go func() {
		for {
			if suspected := n.partitionDetector.DetectPartition(); len(suspected) > 0 {
				n.HandlePartition(suspected)
			}
		}
	}()
}

// HandlePartition 处理检测到的分区
func (n *Node) HandlePartition(suspectedNodes []uint64) {
	// 简化处理：当检测到网络分区时，如果当前是Leader且超过半数节点不可达，则降级
	if n.state == StateLeader {
		quorum := n.quorum()
		if len(suspectedNodes) >= quorum-1 { // -1 排除自己
			n.becomeFollower(n.currentTerm, 0)
		}
	}
}

// TransferLeadership 触发领导权转移
func (n *Node) TransferLeadership(ctx context.Context, targetID uint64) error {
	if n.state != StateLeader {
		return fmt.Errorf("not leader")
	}

	if _, ok := n.peers[targetID]; !ok {
		return fmt.Errorf("unknown target leader: %d", targetID)
	}

	// 向目标节点发送 TimeoutNow 消息
	n.send(Message{
		Type:  MsgTimeoutNow,
		From:  n.id,
		To:    targetID,
		Term:  n.currentTerm,
		Force: true, // 强制立即触发选举
	})

	return nil
}

// handleTimeoutNow 处理立即选举请求
func (n *Node) handleTimeoutNow(msg Message) {
	// 收到 TimeoutNow 消息后，立即发起选举
	if msg.Force {
		n.campaign()
	}
}