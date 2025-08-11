package raft

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMemoryStorage 测试内存存储
func TestMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()

	if storage == nil {
		t.Fatal("Expected storage to be created")
	}

	// 测试初始状态
	hardState, err := storage.InitialState()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !IsEmptyHardState(hardState) {
		t.Error("Expected initial hard state to be empty")
	}

	// 测试初始条目
	entries, err := storage.Entries(1, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected no initial entries, got %d", len(entries))
	}
}

// TestMemoryStorageSetHardState 测试设置硬状态
func TestMemoryStorageSetHardState(t *testing.T) {
	storage := NewMemoryStorage()

	hardState := HardState{
		Term:   5,
		Vote:   2,
		Commit: 10,
	}

	err := storage.SetHardState(hardState)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 验证状态已保存
	retrievedState, err := storage.InitialState()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if retrievedState.Term != hardState.Term {
		t.Errorf("Expected term %d, got %d", hardState.Term, retrievedState.Term)
	}

	if retrievedState.Vote != hardState.Vote {
		t.Errorf("Expected vote %d, got %d", hardState.Vote, retrievedState.Vote)
	}

	if retrievedState.Commit != hardState.Commit {
		t.Errorf("Expected commit %d, got %d", hardState.Commit, retrievedState.Commit)
	}
}

// TestMemoryStorageAppend 测试追加条目
func TestMemoryStorageAppend(t *testing.T) {
	storage := NewMemoryStorage()

	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}

	err := storage.Append(entries)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 验证条目已保存
	retrievedEntries, err := storage.Entries(1, 4)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(retrievedEntries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(retrievedEntries))
	}

	for i, entry := range retrievedEntries {
		if entry.Index != entries[i].Index {
			t.Errorf("Entry %d: expected index %d, got %d", i, entries[i].Index, entry.Index)
		}
		if entry.Term != entries[i].Term {
			t.Errorf("Entry %d: expected term %d, got %d", i, entries[i].Term, entry.Term)
		}
		if string(entry.Data) != string(entries[i].Data) {
			t.Errorf("Entry %d: expected data %s, got %s", i, entries[i].Data, entry.Data)
		}
	}
}

// TestMemoryStorageAppendConflict 测试追加冲突条目
func TestMemoryStorageAppendConflict(t *testing.T) {
	storage := NewMemoryStorage()

	// 先添加一些条目
	initialEntries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	storage.Append(initialEntries)

	// 添加冲突的条目（从索引2开始，不同的任期）
	conflictEntries := []LogEntry{
		{Index: 2, Term: 2, Type: EntryNormal, Data: []byte("new_entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("new_entry3")},
	}

	err := storage.Append(conflictEntries)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 验证冲突的条目已被替换
	retrievedEntries, err := storage.Entries(1, 4)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(retrievedEntries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(retrievedEntries))
	}

	// 检查第一个条目未变
	if retrievedEntries[0].Term != 1 {
		t.Errorf("Expected first entry term to be 1, got %d", retrievedEntries[0].Term)
	}

	// 检查第二个条目已更新
	if retrievedEntries[1].Term != 2 {
		t.Errorf("Expected second entry term to be 2, got %d", retrievedEntries[1].Term)
	}

	if string(retrievedEntries[1].Data) != "new_entry2" {
		t.Errorf("Expected second entry data to be 'new_entry2', got %s", retrievedEntries[1].Data)
	}
}

// TestMemoryStorageTerm 测试获取任期
func TestMemoryStorageTerm(t *testing.T) {
	storage := NewMemoryStorage()

	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	storage.Append(entries)

	tests := []struct {
		index    uint64
		expected uint64
		hasError bool
	}{
		{0, 0, false}, // 索引 0 应该返回 0
		{1, 1, false}, // 第一个条目
		{2, 1, false}, // 第二个条目
		{3, 2, false}, // 第三个条目
		{4, 0, true},  // 超出范围
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			term, err := storage.Term(tt.index)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for index %d", tt.index)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for index %d: %v", tt.index, err)
				}
				if term != tt.expected {
					t.Errorf("For index %d, expected term %d, got %d",
						tt.index, tt.expected, term)
				}
			}
		})
	}
}

// TestMemoryStorageLastIndex 测试获取最后索引
func TestMemoryStorageLastIndex(t *testing.T) {
	storage := NewMemoryStorage()

	// 初始状态
	lastIndex, err := storage.LastIndex()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if lastIndex != 0 {
		t.Errorf("Expected initial last index to be 0, got %d", lastIndex)
	}

	// 添加条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
	}
	storage.Append(entries)

	lastIndex, err = storage.LastIndex()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if lastIndex != 2 {
		t.Errorf("Expected last index to be 2, got %d", lastIndex)
	}
}

// TestMemoryStorageFirstIndex 测试获取第一个索引
func TestMemoryStorageFirstIndex(t *testing.T) {
	storage := NewMemoryStorage()

	// 初始状态
	firstIndex, err := storage.FirstIndex()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if firstIndex != 1 {
		t.Errorf("Expected initial first index to be 1, got %d", firstIndex)
	}

	// 添加条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
	}
	storage.Append(entries)

	firstIndex, err = storage.FirstIndex()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if firstIndex != 1 {
		t.Errorf("Expected first index to be 1, got %d", firstIndex)
	}
}

// TestMemoryStorageSnapshot 测试快照
func TestMemoryStorageSnapshot(t *testing.T) {
	storage := NewMemoryStorage()

	// 初始状态应该没有快照
	snapshot, err := storage.Snapshot()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !IsEmptySnapshot(snapshot) {
		t.Error("Expected initial snapshot to be empty")
	}

	// 先添加一些日志条目，这样才能创建快照
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
		{Index: 4, Term: 2, Type: EntryNormal, Data: []byte("entry4")},
		{Index: 5, Term: 2, Type: EntryNormal, Data: []byte("entry5")},
	}
	err = storage.Append(entries)
	if err != nil {
		t.Fatalf("Failed to append entries: %v", err)
	}

	// 创建快照
	snapshotData := &Snapshot{
		Index: 5,
		Term:  2,
		Data:  []byte("snapshot_data"),
	}

	createdSnapshot, err := storage.CreateSnapshot(snapshotData.Index, snapshotData.Data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 验证快照
	retrievedSnapshot, err := storage.Snapshot()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if retrievedSnapshot.Index != snapshotData.Index {
		t.Errorf("Expected snapshot index %d, got %d", snapshotData.Index, retrievedSnapshot.Index)
	}

	if retrievedSnapshot.Term != snapshotData.Term {
		t.Errorf("Expected snapshot term %d, got %d", snapshotData.Term, retrievedSnapshot.Term)
	}

	if string(retrievedSnapshot.Data) != string(snapshotData.Data) {
		t.Errorf("Expected snapshot data %s, got %s", snapshotData.Data, retrievedSnapshot.Data)
	}

	// 验证创建的快照与检索的快照一致
	if createdSnapshot.Index != retrievedSnapshot.Index {
		t.Errorf("Created and retrieved snapshot index mismatch: %d vs %d",
			createdSnapshot.Index, retrievedSnapshot.Index)
	}

	if createdSnapshot.Term != retrievedSnapshot.Term {
		t.Errorf("Created and retrieved snapshot term mismatch: %d vs %d",
			createdSnapshot.Term, retrievedSnapshot.Term)
	}
}

// TestFileStorage 测试文件存储
func TestFileStorage(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "raft_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	defer storage.Close()

	// 测试初始状态
	hardState, err := storage.InitialState()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !IsEmptyHardState(hardState) {
		t.Error("Expected initial hard state to be empty")
	}
}

// TestFileStoragePersistence 测试文件存储持久化
func TestFileStoragePersistence(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "raft_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建第一个存储实例
	storage1, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	// 设置硬状态
	hardState := HardState{
		Term:   5,
		Vote:   2,
		Commit: 10,
	}
	err = storage1.SetHardState(hardState)
	if err != nil {
		t.Fatalf("Failed to set hard state: %v", err)
	}

	// 添加条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
	}
	err = storage1.Append(entries)
	if err != nil {
		t.Fatalf("Failed to append entries: %v", err)
	}

	storage1.Close()

	// 创建第二个存储实例（重新打开）
	storage2, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to reopen file storage: %v", err)
	}
	defer storage2.Close()

	// 验证硬状态已持久化
	retrievedState, err := storage2.InitialState()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if retrievedState.Term != hardState.Term {
		t.Errorf("Expected term %d, got %d", hardState.Term, retrievedState.Term)
	}

	if retrievedState.Vote != hardState.Vote {
		t.Errorf("Expected vote %d, got %d", hardState.Vote, retrievedState.Vote)
	}

	if retrievedState.Commit != hardState.Commit {
		t.Errorf("Expected commit %d, got %d", hardState.Commit, retrievedState.Commit)
	}

	// 验证条目已持久化
	retrievedEntries, err := storage2.Entries(1, 3)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(retrievedEntries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(retrievedEntries))
	}

	for i, entry := range retrievedEntries {
		if entry.Index != entries[i].Index {
			t.Errorf("Entry %d: expected index %d, got %d", i, entries[i].Index, entry.Index)
		}
		if string(entry.Data) != string(entries[i].Data) {
			t.Errorf("Entry %d: expected data %s, got %s", i, entries[i].Data, entry.Data)
		}
	}
}

// TestFileStorageCorruption 测试文件存储损坏处理
func TestFileStorageCorruption(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "raft_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建损坏的状态文件
	stateFile := filepath.Join(tempDir, "state.json")
	err = os.WriteFile(stateFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	// 尝试打开存储（应该处理损坏的文件）
	storage, err := NewFileStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file storage with corrupted file: %v", err)
	}
	defer storage.Close()

	// 应该返回空的初始状态
	hardState, err := storage.InitialState()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !IsEmptyHardState(hardState) {
		t.Error("Expected empty hard state when file is corrupted")
	}
}

// TestStorageInterface 测试存储接口一致性
func TestStorageInterface(t *testing.T) {
	storages := []struct {
		name    string
		storage Storage
		cleanup func()
	}{
		{
			name:    "MemoryStorage",
			storage: NewMemoryStorage(),
			cleanup: func() {},
		},
	}

	// 如果文件存储可用，添加到测试中
	tempDir, err := os.MkdirTemp("", "raft_test")
	if err == nil {
		fileStorage, err := NewFileStorage(tempDir)
		if err == nil {
			storages = append(storages, struct {
				name    string
				storage Storage
				cleanup func()
			}{
				name:    "FileStorage",
				storage: fileStorage,
				cleanup: func() {
					fileStorage.Close()
					os.RemoveAll(tempDir)
				},
			})
		}
	}

	for _, s := range storages {
		t.Run(s.name, func(t *testing.T) {
			defer s.cleanup()

			// 测试基本操作
			testStorageBasicOperations(t, s.storage)
		})
	}
}

// testStorageBasicOperations 测试存储的基本操作
func testStorageBasicOperations(t *testing.T, storage Storage) {
	// 测试初始状态
	hardState, err := storage.InitialState()
	if err != nil {
		t.Fatalf("InitialState failed: %v", err)
	}

	if !IsEmptyHardState(hardState) {
		t.Error("Expected initial hard state to be empty")
	}

	// 对于具体的存储实现，测试设置硬状态和追加条目
	if ms, ok := storage.(*MemoryStorage); ok {
		// 测试设置硬状态
		newHardState := HardState{Term: 1, Vote: 1, Commit: 0}
		err = ms.SetHardState(newHardState)
		if err != nil {
			t.Fatalf("SetHardState failed: %v", err)
		}

		// 测试追加条目
		entries := []LogEntry{
			{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("test")},
		}
		err = ms.Append(entries)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	} else if fs, ok := storage.(*FileStorage); ok {
		// 测试设置硬状态
		newHardState := HardState{Term: 1, Vote: 1, Commit: 0}
		err = fs.SetHardState(newHardState)
		if err != nil {
			t.Fatalf("SetHardState failed: %v", err)
		}

		// 测试追加条目
		entries := []LogEntry{
			{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("test")},
		}
		err = fs.Append(entries)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// 测试获取条目
	retrievedEntries, err := storage.Entries(1, 2)
	if err != nil {
		t.Fatalf("Entries failed: %v", err)
	}

	if len(retrievedEntries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(retrievedEntries))
	}

	// 测试获取任期
	term, err := storage.Term(1)
	if err != nil {
		t.Fatalf("Term failed: %v", err)
	}

	if term != 1 {
		t.Errorf("Expected term 1, got %d", term)
	}

	// 测试获取索引范围
	firstIndex, err := storage.FirstIndex()
	if err != nil {
		t.Fatalf("FirstIndex failed: %v", err)
	}

	lastIndex, err := storage.LastIndex()
	if err != nil {
		t.Fatalf("LastIndex failed: %v", err)
	}

	if firstIndex != 1 {
		t.Errorf("Expected first index 1, got %d", firstIndex)
	}

	if lastIndex != 1 {
		t.Errorf("Expected last index 1, got %d", lastIndex)
	}
}
