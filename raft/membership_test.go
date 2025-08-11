package raft

import (
	"context"
	"testing"
	"time"
)

// TestMembershipManagerCreation 测试成员管理器创建
func TestMembershipManagerCreation(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)

	mm := NewMembershipManager(node)

	if mm == nil {
		t.Fatal("Expected membership manager to be created")
	}

	if mm.node != node {
		t.Error("Expected node to be set correctly")
	}

	if mm.members == nil {
		t.Error("Expected members map to be initialized")
	}

	if mm.confChangeTimeout != 30*time.Second {
		t.Errorf("Expected default confChangeTimeout to be 30 seconds, got %v", mm.confChangeTimeout)
	}
}

// TestMembershipManagerAddMember 测试添加成员
func TestMembershipManagerAddMember(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 测试添加新成员
	err := mm.AddMember(ctx, 2, "node2")
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// 验证成员已添加
	members := mm.GetMembers()
	if len(members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(members))
	}

	if member, exists := members[2]; !exists {
		t.Error("Expected member 2 to exist")
	} else if member.Address != "node2" {
		t.Errorf("Expected member address to be 'node2', got '%s'", member.Address)
	}
}

// TestMembershipManagerAddDuplicateMember 测试添加重复成员
func TestMembershipManagerAddDuplicateMember(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 添加成员
	err := mm.AddMember(ctx, 2, "node2")
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// 尝试添加相同ID的成员
	err = mm.AddMember(ctx, 2, "node2-duplicate")
	if err == nil {
		t.Error("Expected error when adding duplicate member")
	}

	// 验证原成员未被修改
	members := mm.GetMembers()
	if member, exists := members[2]; !exists {
		t.Error("Expected original member to still exist")
	} else if member.Address != "node2" {
		t.Errorf("Expected original member address, got '%s'", member.Address)
	}
}

// TestMembershipManagerRemoveMember 测试移除成员
func TestMembershipManagerRemoveMember(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.ID = 1
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)

	// 设置为 Leader 状态以便能够提议配置变更
	node.state = StateLeader

	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 先添加成员
	mm.AddMember(ctx, 2, "node2")
	mm.ClearPendingConfChange() // 清除 pending 状态
	mm.AddMember(ctx, 3, "node3")
	mm.ClearPendingConfChange() // 清除 pending 状态

	// 移除成员
	err := mm.RemoveMember(ctx, 2)
	if err != nil {
		t.Fatalf("Failed to remove member: %v", err)
	}

	// 验证成员状态已更新为 Leaving
	members := mm.GetMembers()
	if member, exists := members[2]; !exists {
		t.Error("Expected member 2 to still exist with Leaving status")
	} else if member.Status != MemberStatusLeaving {
		t.Errorf("Expected member 2 status to be Leaving, got %v", member.Status)
	}

	if _, exists := members[3]; !exists {
		t.Error("Expected member 3 to still exist")
	}
}

// TestMembershipManagerRemoveNonexistentMember 测试移除不存在的成员
func TestMembershipManagerRemoveNonexistentMember(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 尝试移除不存在的成员
	err := mm.RemoveMember(ctx, 999)
	if err == nil {
		t.Error("Expected error when removing nonexistent member")
	}
}

// TestMembershipManagerApplyConfChange 测试应用配置变更
func TestMembershipManagerApplyConfChange(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	// 测试添加节点的配置变更
	addChange := ConfChange{
		Type:    ConfChangeAddNode,
		NodeID:  2,
		Context: []byte(`{"address":"node2"}`),
	}

	confState := mm.ApplyConfChange(addChange)

	// 验证配置状态
	if len(confState.Nodes) != 2 { // 包括初始节点
		t.Errorf("Expected 2 nodes in confState, got %d", len(confState.Nodes))
	}

	// 验证成员已添加
	members := mm.GetMembers()
	if member, exists := members[2]; !exists {
		t.Error("Expected member 2 to be added via ConfChange")
	} else if member.Status != MemberStatusActive {
		t.Errorf("Expected member 2 status to be Active, got %v", member.Status)
	}

	// 测试移除节点的配置变更
	removeChange := ConfChange{
		Type:   ConfChangeRemoveNode,
		NodeID: 2,
	}

	confState = mm.ApplyConfChange(removeChange)

	// 验证配置状态
	if len(confState.Nodes) != 1 {
		t.Errorf("Expected 1 node in confState after removal, got %d", len(confState.Nodes))
	}

	// 验证成员状态已更新
	members = mm.GetMembers()
	if member, exists := members[2]; !exists {
		t.Error("Expected member 2 to still exist")
	} else if member.Status != MemberStatusLeft {
		t.Errorf("Expected member 2 status to be Left, got %v", member.Status)
	}
}

// TestMembershipManagerGetMembers 测试获取成员列表
func TestMembershipManagerGetMembers(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)

	// 设置为 Leader 状态以便能够提议配置变更
	node.state = StateLeader

	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 初始状态应该为空
	members := mm.GetMembers()
	if len(members) != 0 {
		t.Errorf("Expected 0 members initially, got %d", len(members))
	}

	// 添加一些成员
	mm.AddMember(ctx, 2, "node2")
	mm.ClearPendingConfChange() // 清除 pending 状态
	mm.AddMember(ctx, 3, "node3")
	mm.ClearPendingConfChange() // 清除 pending 状态
	mm.AddMember(ctx, 4, "node4")
	mm.ClearPendingConfChange() // 清除 pending 状态

	// 验证成员列表
	members = mm.GetMembers()
	if len(members) != 3 {
		t.Errorf("Expected 3 members, got %d", len(members))
	}

	expectedMembers := map[uint64]string{
		2: "node2",
		3: "node3",
		4: "node4",
	}

	for id, expectedAddr := range expectedMembers {
		if member, exists := members[id]; !exists {
			t.Errorf("Expected member %d to exist", id)
		} else if member.Address != expectedAddr {
			t.Errorf("Expected member %d address to be '%s', got '%s'", id, expectedAddr, member.Address)
		}
	}
}

// TestMembershipManagerConfigChangeInProgress 测试配置变更进行中的状态
func TestMembershipManagerConfigChangeInProgress(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)

	// 设置为 Leader 状态以便能够提议配置变更
	node.state = StateLeader

	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 初始状态
	if mm.HasPendingConfChange() {
		t.Error("Expected no pending conf change initially")
	}

	// 添加成员会创建 pending conf change
	err := mm.AddMember(ctx, 2, "node2")
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// 现在应该有 pending conf change
	if !mm.HasPendingConfChange() {
		t.Error("Expected pending conf change after adding member")
	}

	// 在配置变更进行中时，不应允许新的配置变更
	err = mm.AddMember(ctx, 3, "node3")
	if err == nil {
		t.Error("Expected error when adding member during config change")
	}

	err = mm.RemoveMember(ctx, 2)
	if err == nil {
		t.Error("Expected error when removing member during config change")
	}

	// 清除 pending conf change
	mm.ClearPendingConfChange()

	// 现在应该能够进行配置变更
	err = mm.AddMember(ctx, 3, "node3")
	if err != nil {
		t.Errorf("Failed to add member after clearing pending conf change: %v", err)
	}
}

// TestMembershipManagerConcurrentAccess 测试并发访问
func TestMembershipManagerConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 并发获取成员和清除 pending changes
	done := make(chan bool, 3)

	go func() {
		for i := 0; i < 20; i++ {
			mm.GetMembers()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			mm.HasPendingConfChange()
			mm.ClearPendingConfChange()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	go func() {
		// 尝试添加一些成员
		for i := 10; i < 15; i++ {
			mm.AddMember(ctx, uint64(i), "node"+string(rune(i+'0')))
			time.Sleep(time.Millisecond * 2)
		}
		done <- true
	}()

	// 等待所有goroutine完成
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(time.Second * 2):
			t.Fatal("Concurrent access test timed out")
		}
	}

	// 验证最终状态
	members := mm.GetMembers()
	if len(members) < 1 {
		t.Errorf("Expected at least 1 member after concurrent operations, got %d", len(members))
	}
}

// TestMembershipManagerMemberInfo 测试成员信息结构
func TestMembershipManagerMemberInfo(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 添加成员
	err := mm.AddMember(ctx, 2, "192.168.1.2:8080")
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// 验证成员信息
	members := mm.GetMembers()
	member := members[2]

	if member.ID != 2 {
		t.Errorf("Expected member ID to be 2, got %d", member.ID)
	}

	if member.Address != "192.168.1.2:8080" {
		t.Errorf("Expected member address to be '192.168.1.2:8080', got '%s'", member.Address)
	}

	if member.Status != MemberStatusJoining {
		t.Errorf("Expected member status to be Joining, got %v", member.Status)
	}

	// 验证JoinTime时间已设置
	if member.JoinTime.IsZero() {
		t.Error("Expected JoinTime to be set")
	}

	// 验证JoinTime时间在合理范围内（过去1秒内）
	if time.Since(member.JoinTime) > time.Second {
		t.Error("JoinTime time seems too old")
	}
}

// TestMembershipManagerGetActiveMembers 测试获取活跃成员
func TestMembershipManagerGetActiveMembers(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)
	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 添加成员
	mm.AddMember(ctx, 2, "node2")
	mm.AddMember(ctx, 3, "node3")

	// 通过 ConfChange 激活一个成员
	confChange := ConfChange{
		Type:    ConfChangeAddNode,
		NodeID:  2,
		Context: []byte(`{"address":"node2"}`),
	}
	mm.ApplyConfChange(confChange)

	// 获取活跃成员
	activeMembers := mm.GetActiveMembers()
	if len(activeMembers) != 1 {
		t.Errorf("Expected 1 active member, got %d", len(activeMembers))
	}

	if _, exists := activeMembers[2]; !exists {
		t.Error("Expected member 2 to be active")
	}

	if _, exists := activeMembers[3]; exists {
		t.Error("Expected member 3 to not be active (still joining)")
	}
}

// TestMembershipManagerClusterInfo 测试集群信息
func TestMembershipManagerClusterInfo(t *testing.T) {
	storage := NewMemoryStorage()
	config := DefaultConfig()
	config.Storage = storage
	sm := &testStateMachine{data: make(map[string]string)}
	node := NewNode(config, sm)

	// 设置为 Leader 状态以测试 Leader 检测
	node.state = StateLeader

	mm := NewMembershipManager(node)

	ctx := context.Background()

	// 添加并激活一些成员
	mm.AddMember(ctx, 2, "node2")
	confChange := ConfChange{
		Type:    ConfChangeAddNode,
		NodeID:  2,
		Context: []byte(`{"address":"node2"}`),
	}
	mm.ApplyConfChange(confChange)

	// 获取集群信息
	info := mm.GetClusterInfo()

	if info.Leader != node.id {
		t.Errorf("Expected leader to be %d, got %d", node.id, info.Leader)
	}

	if info.ClusterSize != 1 { // 只有一个活跃成员
		t.Errorf("Expected cluster size to be 1, got %d", info.ClusterSize)
	}

	if info.QuorumSize != 1 {
		t.Errorf("Expected quorum size to be 1, got %d", info.QuorumSize)
	}
}
