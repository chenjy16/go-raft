package raft

import (
	"context"
	"testing"
	"time"
)

// TestSnapshotManagerCreation 测试快照管理器创建
func TestSnapshotManagerCreation(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	if sm == nil {
		t.Fatal("Expected snapshot manager to be created")
	}

	if sm.node != node {
		t.Error("Expected node to be set correctly")
	}

	if sm.storage != storage {
		t.Error("Expected storage to be set correctly")
	}

	if sm.stateMachine != stateMachine {
		t.Error("Expected state machine to be set correctly")
	}

	if sm.snapshotThreshold != 1000 {
		t.Errorf("Expected default snapshot threshold to be 1000, got %d", sm.snapshotThreshold)
	}

	if sm.compactThreshold != 10000 {
		t.Errorf("Expected default compact threshold to be 10000, got %d", sm.compactThreshold)
	}
}

// TestSnapshotManagerSetThresholds 测试设置阈值
func TestSnapshotManagerSetThresholds(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 测试设置快照阈值
	sm.SetSnapshotThreshold(500)
	if sm.snapshotThreshold != 500 {
		t.Errorf("Expected snapshot threshold to be 500, got %d", sm.snapshotThreshold)
	}

	// 测试设置压缩阈值
	sm.SetCompactThreshold(5000)
	if sm.compactThreshold != 5000 {
		t.Errorf("Expected compact threshold to be 5000, got %d", sm.compactThreshold)
	}
}

// TestSnapshotManagerStartStop 测试启动和停止
func TestSnapshotManagerStartStop(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 启动快照管理器
	sm.Start()

	// 等待一小段时间确保goroutine启动
	time.Sleep(10 * time.Millisecond)

	// 停止快照管理器
	sm.Stop()

	// 验证已停止（通过尝试再次停止来验证）
	done := make(chan struct{})
	go func() {
		sm.Stop() // 这应该不会阻塞，因为已经停止了
		close(done)
	}()

	select {
	case <-done:
		// 正常停止
	case <-time.After(100 * time.Millisecond):
		t.Error("Stop() blocked, indicating the manager wasn't properly stopped")
	}
}

// TestSnapshotManagerCreateSnapshotNow 测试立即创建快照
func TestSnapshotManagerCreateSnapshotNow(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	// 添加一些日志条目到存储
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
	}
	storage.Append(entries)

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 立即创建快照
	err := sm.CreateSnapshotNow()
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// 验证快照信息已更新
	lastSnapshotIndex, _ := sm.GetSnapshotInfo()
	if lastSnapshotIndex == 0 {
		t.Error("Expected last snapshot index to be updated")
	}
}

// TestSnapshotManagerCompactLogNow 测试立即压缩日志
func TestSnapshotManagerCompactLogNow(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	// 添加一些日志条目到存储
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 1, Type: EntryNormal, Data: []byte("entry3")},
	}
	storage.Append(entries)

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 立即压缩日志到索引2
	err := sm.CompactLogNow(2)
	if err != nil {
		t.Fatalf("Failed to compact log: %v", err)
	}

	// 验证压缩信息已更新
	_, lastCompactIndex := sm.GetSnapshotInfo()
	if lastCompactIndex != 2 {
		t.Errorf("Expected last compact index to be 2, got %d", lastCompactIndex)
	}
}

// TestSnapshotManagerRestoreFromSnapshot 测试从快照恢复
func TestSnapshotManagerRestoreFromSnapshot(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 创建测试快照
	snapshotData := []byte(`{"key1":"value1","key2":"value2"}`)
	snapshot := &Snapshot{
		Index: 5,
		Term:  2,
		Data:  snapshotData,
	}

	// 从快照恢复
	err := sm.RestoreFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("Failed to restore from snapshot: %v", err)
	}

	// 验证快照信息已更新
	lastSnapshotIndex, _ := sm.GetSnapshotInfo()
	if lastSnapshotIndex != 5 {
		t.Errorf("Expected last snapshot index to be 5, got %d", lastSnapshotIndex)
	}
}

// TestSnapshotManagerGetSnapshotInfo 测试获取快照信息
func TestSnapshotManagerGetSnapshotInfo(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 初始状态
	snapshotIndex, compactIndex := sm.GetSnapshotInfo()
	if snapshotIndex != 0 {
		t.Errorf("Expected initial snapshot index to be 0, got %d", snapshotIndex)
	}
	if compactIndex != 0 {
		t.Errorf("Expected initial compact index to be 0, got %d", compactIndex)
	}

	// 设置一些值
	sm.lastSnapshotIndex = 10
	sm.lastCompactIndex = 5

	// 再次检查
	snapshotIndex, compactIndex = sm.GetSnapshotInfo()
	if snapshotIndex != 10 {
		t.Errorf("Expected snapshot index to be 10, got %d", snapshotIndex)
	}
	if compactIndex != 5 {
		t.Errorf("Expected compact index to be 5, got %d", compactIndex)
	}
}

// TestSnapshotManagerAutomaticSnapshot 测试自动快照
func TestSnapshotManagerAutomaticSnapshot(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 设置较小的快照阈值以便测试
	sm.SetSnapshotThreshold(2)

	// 添加超过阈值的日志条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 1, Type: EntryNormal, Data: []byte("entry3")},
	}
	storage.Append(entries)

	// 手动触发检查
	sm.checkAndCreateSnapshot()

	// 验证快照已创建
	lastSnapshotIndex, _ := sm.GetSnapshotInfo()
	if lastSnapshotIndex == 0 {
		t.Error("Expected automatic snapshot to be created")
	}
}

// TestSnapshotManagerAutomaticCompaction 测试自动压缩
func TestSnapshotManagerAutomaticCompaction(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 设置较小的压缩阈值以便测试
	sm.SetCompactThreshold(3)

	// 添加超过阈值的日志条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 1, Type: EntryNormal, Data: []byte("entry3")},
		{Index: 4, Term: 1, Type: EntryNormal, Data: []byte("entry4")},
		{Index: 5, Term: 1, Type: EntryNormal, Data: []byte("entry5")},
	}
	storage.Append(entries)

	// 先创建一个快照作为压缩基础
	sm.lastSnapshotIndex = 3

	// 手动触发检查
	sm.checkAndCompactLog()

	// 验证压缩已执行
	_, lastCompactIndex := sm.GetSnapshotInfo()
	if lastCompactIndex == 0 {
		t.Error("Expected automatic compaction to be performed")
	}
}

// TestSnapshotManagerConcurrentAccess 测试并发访问
func TestSnapshotManagerConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()
	stateMachine := &testStateMachine{data: make(map[string]string)}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, stateMachine)

	// 添加一些日志条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
	}
	storage.Append(entries)

	// 并发执行多个操作
	done := make(chan bool, 4)

	// 并发设置阈值
	go func() {
		for i := 0; i < 10; i++ {
			sm.SetSnapshotThreshold(uint64(100 + i))
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// 并发获取信息
	go func() {
		for i := 0; i < 10; i++ {
			sm.GetSnapshotInfo()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// 并发创建快照
	go func() {
		for i := 0; i < 5; i++ {
			sm.CreateSnapshotNow()
			time.Sleep(2 * time.Millisecond)
		}
		done <- true
	}()

	// 并发压缩
	go func() {
		for i := 0; i < 5; i++ {
			sm.CompactLogNow(1)
			time.Sleep(2 * time.Millisecond)
		}
		done <- true
	}()

	// 等待所有goroutine完成
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Concurrent access test timed out")
		}
	}
}

// MockStateMachine 模拟状态机，用于测试
type MockStateMachine struct {
	data        map[string]string
	snapshotErr error
	restoreErr  error
}

func (m *MockStateMachine) Apply(data []byte) ([]byte, error) {
	return []byte("ok"), nil
}

func (m *MockStateMachine) Snapshot() ([]byte, error) {
	if m.snapshotErr != nil {
		return nil, m.snapshotErr
	}
	return []byte(`{"test":"data"}`), nil
}

func (m *MockStateMachine) Restore(data []byte) error {
	return m.restoreErr
}

// TestSnapshotManagerWithFailingStateMachine 测试状态机失败的情况
func TestSnapshotManagerWithFailingStateMachine(t *testing.T) {
	storage := NewMemoryStorage()
	mockSM := &MockStateMachine{
		data:        make(map[string]string),
		snapshotErr: context.DeadlineExceeded, // 模拟快照失败
	}
	node := &Node{id: 1}

	sm := NewSnapshotManager(node, storage, mockSM)

	// 添加一些日志条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
	}
	storage.Append(entries)

	// 尝试创建快照，应该失败
	err := sm.CreateSnapshotNow()
	if err == nil {
		t.Error("Expected snapshot creation to fail")
	}

	// 验证快照索引没有更新
	lastSnapshotIndex, _ := sm.GetSnapshotInfo()
	if lastSnapshotIndex != 0 {
		t.Errorf("Expected snapshot index to remain 0, got %d", lastSnapshotIndex)
	}
}
