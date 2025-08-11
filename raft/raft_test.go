package raft

import (
	"context"
	"testing"
	"time"
)

// TestNodeCreation 测试节点创建
func TestNodeCreation(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	if node == nil {
		t.Fatal("Expected node to be created")
	}
	
	if node.id != config.ID {
		t.Errorf("Expected node ID %d, got %d", config.ID, node.id)
	}
	
	if node.state != StateFollower {
		t.Errorf("Expected initial state to be Follower, got %v", node.state)
	}
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.ID != 1 {
		t.Errorf("Expected default ID to be 1, got %d", config.ID)
	}
	
	if config.ElectionTick != 10 {
		t.Errorf("Expected default ElectionTick to be 10, got %d", config.ElectionTick)
	}
	
	if config.HeartbeatTick != 1 {
		t.Errorf("Expected default HeartbeatTick to be 1, got %d", config.HeartbeatTick)
	}
}

// TestNodeStateTransitions 测试节点状态转换
func TestNodeStateTransitions(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 测试从 Follower 到 Candidate
	node.becomeCandidate()
	if node.state != StateCandidate {
		t.Errorf("Expected state to be Candidate, got %v", node.state)
	}
	
	// 测试从 Candidate 到 Leader
	node.becomeLeader()
	if node.state != StateLeader {
		t.Errorf("Expected state to be Leader, got %v", node.state)
	}
	
	// 测试从 Leader 到 Follower
	node.becomeFollower(2, 0)
	if node.state != StateFollower {
		t.Errorf("Expected state to be Follower, got %v", node.state)
	}
	
	if node.currentTerm != 2 {
		t.Errorf("Expected term to be 2, got %d", node.currentTerm)
	}
}

// TestElectionTimeout 测试选举超时
func TestElectionTimeout(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 模拟选举超时
	for i := 0; i < config.ElectionTick; i++ {
		node.tick()
	}
	
	if node.state != StateCandidate {
		t.Errorf("Expected node to become candidate after election timeout")
	}
}

// TestVoteRequest 测试投票请求
func TestVoteRequest(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 创建投票请求消息
	msg := Message{
		Type:     MsgRequestVote,
		From:     2,
		To:       1,
		Term:     1,
		LogIndex: 0,
		LogTerm:  0,
	}
	
	node.step(msg)
	
	// 检查是否发送了投票响应
	if len(node.msgs) == 0 {
		t.Fatal("Expected vote response message")
	}
	
	resp := node.msgs[0]
	if resp.Type != MsgRequestVoteResp {
		t.Errorf("Expected MsgRequestVoteResp, got %v", resp.Type)
	}
	
	if !resp.VoteGranted {
		t.Error("Expected vote to be granted")
	}
}

// TestLogReplication 测试日志复制
func TestLogReplication(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 成为 Leader
	node.becomeLeader()
	
	// 添加一个 peer
	node.peers[2] = true
	node.nextIndex[2] = 1
	node.matchIndex[2] = 0
	
	// 提议一个日志条目
	data := []byte("test data")
	err := node.Propose(context.Background(), data)
	if err != nil {
		t.Fatalf("Failed to propose: %v", err)
	}
	
	// 检查日志是否被添加
	lastIndex := node.raftLog.LastIndex()
	if lastIndex == 0 {
		t.Error("Expected log entry to be added")
	}
	
	// 检查是否发送了 AppendEntries 消息
	if len(node.msgs) == 0 {
		t.Fatal("Expected AppendEntries message")
	}
	
	msg := node.msgs[0]
	if msg.Type != MsgAppendEntries {
		t.Errorf("Expected MsgAppendEntries, got %v", msg.Type)
	}
}

// TestCommitIndex 测试提交索引更新
func TestCommitIndex(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 成为 Leader
	node.becomeLeader()
	
	// 添加 peers
	node.peers[2] = true
	node.peers[3] = true
	node.nextIndex[2] = 1
	node.nextIndex[3] = 1
	node.matchIndex[2] = 0
	node.matchIndex[3] = 0
	
	// 添加日志条目
	entry := LogEntry{
		Index: 1,
		Term:  1,
		Type:  EntryNormal,
		Data:  []byte("test"),
	}
	node.raftLog.Append(entry)
	
	// 模拟大多数节点确认
	node.matchIndex[2] = 1
	node.matchIndex[3] = 1
	
	// 尝试提交
	node.maybeCommit()
	
	if node.commitIndex != 1 {
		t.Errorf("Expected commit index to be 1, got %d", node.commitIndex)
	}
}

// TestHardStateIsEmpty 测试 HardState 空检查
func TestHardStateIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		state    HardState
		expected bool
	}{
		{
			name:     "empty state",
			state:    HardState{},
			expected: true,
		},
		{
			name:     "non-empty term",
			state:    HardState{Term: 1},
			expected: false,
		},
		{
			name:     "non-empty vote",
			state:    HardState{Vote: 1},
			expected: false,
		},
		{
			name:     "non-empty commit",
			state:    HardState{Commit: 1},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmptyHardState(tt.state)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestSnapshotIsEmpty 测试快照空检查
func TestSnapshotIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *Snapshot
		expected bool
	}{
		{
			name:     "nil snapshot",
			snapshot: nil,
			expected: true,
		},
		{
			name:     "empty snapshot",
			snapshot: &Snapshot{},
			expected: true,
		},
		{
			name:     "non-empty snapshot",
			snapshot: &Snapshot{Index: 1, Term: 1},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmptySnapshot(tt.snapshot)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestMessageTypes 测试消息类型
func TestMessageTypes(t *testing.T) {
	tests := []struct {
		msgType MessageType
		name    string
	}{
		{MsgHeartbeat, "MsgHeartbeat"},
		{MsgHeartbeatResp, "MsgHeartbeatResp"},
		{MsgAppendEntries, "MsgAppendEntries"},
		{MsgAppendEntriesResp, "MsgAppendEntriesResp"},
		{MsgRequestVote, "MsgRequestVote"},
		{MsgRequestVoteResp, "MsgRequestVoteResp"},
		{MsgPropose, "MsgPropose"},
		{MsgSnapshot, "MsgSnapshot"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msgType.String() != tt.name {
				t.Errorf("Expected %s, got %s", tt.name, tt.msgType.String())
			}
		})
	}
}

// TestNodeStates 测试节点状态字符串表示
func TestNodeStates(t *testing.T) {
	tests := []struct {
		state    NodeState
		expected string
	}{
		{StateFollower, "Follower"},
		{StateCandidate, "Candidate"},
		{StateLeader, "Leader"},
		{StateLearner, "Learner"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
			}
		})
	}
}

// testStateMachine 测试用状态机
type testStateMachine struct {
	data map[string]string
}

func (sm *testStateMachine) Apply(data []byte) ([]byte, error) {
	if sm.data == nil {
		sm.data = make(map[string]string)
	}
	// 简单的键值对格式：key=value
	str := string(data)
	sm.data[str] = str
	return data, nil
}

func (sm *testStateMachine) Snapshot() ([]byte, error) {
	return []byte("snapshot"), nil
}

func (sm *testStateMachine) Restore(data []byte) error {
	sm.data = make(map[string]string)
	return nil
}

// TestQuorum 测试法定人数计算
func TestQuorum(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	tests := []struct {
		peers    int
		expected int
	}{
		{1, 1}, // 单节点
		{3, 2}, // 三节点
		{5, 3}, // 五节点
		{7, 4}, // 七节点
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			// 设置 peers
			node.peers = make(map[uint64]bool)
			for i := 1; i <= tt.peers; i++ {
				node.peers[uint64(i)] = true
			}
			
			result := node.quorum()
			if result != tt.expected {
				t.Errorf("For %d peers, expected quorum %d, got %d", 
					tt.peers, tt.expected, result)
			}
		})
	}
}

// TestReady 测试 Ready 状态
func TestReady(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 添加一些消息
	node.send(Message{Type: MsgHeartbeat, From: 1, To: 2})
	
	if !node.hasReady() {
		t.Error("Expected node to have ready state")
	}
	
	ready := node.newReady()
	if len(ready.Messages) == 0 {
		t.Error("Expected ready to have messages")
	}
}

// TestNodeStartStop 测试节点启动和停止
func TestNodeStartStop(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 启动节点
	err := node.Start()
	if err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}
	
	// 等待一小段时间确保节点运行
	time.Sleep(10 * time.Millisecond)
	
	// 停止节点
	node.Stop()
	
	// 验证节点已停止
	select {
	case <-node.done:
		// 节点已正确停止
	case <-time.After(time.Second):
		t.Error("Node did not stop within timeout")
	}
}

// TestProposeAfterStop 测试停止后的提议
func TestProposeAfterStop(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	
	sm := &testStateMachine{}
	node := NewNode(config, sm)
	
	// 启动并立即停止
	node.Start()
	node.Stop()
	
	// 尝试提议
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	err := node.Propose(ctx, []byte("test"))
	if err == nil {
		t.Error("Expected error when proposing to stopped node")
	}
}