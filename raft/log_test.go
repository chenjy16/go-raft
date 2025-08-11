package raft

import (
	"testing"
)

// TestRaftLogCreation 测试 RaftLog 创建
func TestRaftLogCreation(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	if log == nil {
		t.Fatal("Expected log to be created")
	}
	
	if log.storage != storage {
		t.Error("Expected storage to be set")
	}
	
	if log.committed != 0 {
		t.Errorf("Expected committed to be 0, got %d", log.committed)
	}
	
	if log.applied != 0 {
		t.Errorf("Expected applied to be 0, got %d", log.applied)
	}
}

// TestRaftLogCreationWithNilStorage 测试使用 nil storage 创建 RaftLog
func TestRaftLogCreationWithNilStorage(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when creating log with nil storage")
		}
	}()
	
	NewRaftLog(nil)
}

// TestRaftLogAppend 测试日志追加
func TestRaftLogAppend(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 追加单个条目
	entry1 := LogEntry{
		Index: 1,
		Term:  1,
		Type:  EntryNormal,
		Data:  []byte("entry1"),
	}
	log.Append(entry1)
	
	if log.LastIndex() != 1 {
		t.Errorf("Expected last index to be 1, got %d", log.LastIndex())
	}
	
	// 追加多个条目
	entry2 := LogEntry{
		Index: 2,
		Term:  1,
		Type:  EntryNormal,
		Data:  []byte("entry2"),
	}
	entry3 := LogEntry{
		Index: 3,
		Term:  2,
		Type:  EntryNormal,
		Data:  []byte("entry3"),
	}
	log.Append(entry2, entry3)
	
	if log.LastIndex() != 3 {
		t.Errorf("Expected last index to be 3, got %d", log.LastIndex())
	}
}

// TestRaftLogAppendEmpty 测试追加空条目
func TestRaftLogAppendEmpty(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	initialIndex := log.LastIndex()
	log.Append() // 追加空条目
	
	if log.LastIndex() != initialIndex {
		t.Errorf("Expected last index to remain %d, got %d", initialIndex, log.LastIndex())
	}
}

// TestRaftLogTerm 测试获取指定索引的任期
func TestRaftLogTerm(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	
	tests := []struct {
		index    uint64
		expected uint64
		hasError bool
	}{
		{0, 0, false},  // 索引 0 应该返回 0
		{1, 1, false},  // 第一个条目
		{2, 1, false},  // 第二个条目
		{3, 2, false},  // 第三个条目
		{4, 0, true},   // 超出范围
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			term, err := log.Term(tt.index)
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

// TestRaftLogEntries 测试获取日志条目
func TestRaftLogEntries(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
		{Index: 4, Term: 2, Type: EntryNormal, Data: []byte("entry4")},
	}
	log.Append(entries...)
	
	tests := []struct {
		name     string
		lo       uint64
		hi       uint64
		expected int
		hasError bool
	}{
		{"all entries", 1, 5, 4, false},
		{"partial entries", 2, 4, 2, false},
		{"single entry", 3, 4, 1, false},
		{"empty range", 2, 2, 0, false},
		{"invalid range", 5, 6, 0, true},
		{"reverse range", 3, 2, 0, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := log.Entries(tt.lo, tt.hi)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for range [%d, %d)", tt.lo, tt.hi)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for range [%d, %d): %v", tt.lo, tt.hi, err)
				}
				if len(result) != tt.expected {
					t.Errorf("For range [%d, %d), expected %d entries, got %d", 
						tt.lo, tt.hi, tt.expected, len(result))
				}
			}
		})
	}
}

// TestRaftLogMatchTerm 测试任期匹配
func TestRaftLogMatchTerm(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	
	tests := []struct {
		index    uint64
		term     uint64
		expected bool
	}{
		{0, 0, true},   // 索引 0 总是匹配
		{1, 1, true},   // 正确匹配
		{2, 1, true},   // 正确匹配
		{3, 2, true},   // 正确匹配
		{1, 2, false},  // 任期不匹配
		{3, 1, false},  // 任期不匹配
		{4, 1, false},  // 索引超出范围
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := log.MatchTerm(tt.index, tt.term)
			if result != tt.expected {
				t.Errorf("For index %d term %d, expected %v, got %v", 
					tt.index, tt.term, tt.expected, result)
			}
		})
	}
}

// TestRaftLogCommitTo 测试提交
func TestRaftLogCommitTo(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	
	// 测试正常提交
	log.CommitTo(2)
	if log.committed != 2 {
		t.Errorf("Expected committed to be 2, got %d", log.committed)
	}
	
	// 测试提交到更高的索引
	log.CommitTo(3)
	if log.committed != 3 {
		t.Errorf("Expected committed to be 3, got %d", log.committed)
	}
	
	// 测试提交到较低的索引（应该不变）
	log.CommitTo(1)
	if log.committed != 3 {
		t.Errorf("Expected committed to remain 3, got %d", log.committed)
	}
}

// TestRaftLogCommitToOutOfRange 测试提交超出范围的索引
func TestRaftLogCommitToOutOfRange(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一个条目
	entry := LogEntry{
		Index: 1,
		Term:  1,
		Type:  EntryNormal,
		Data:  []byte("entry1"),
	}
	log.Append(entry)
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when committing out of range index")
		}
	}()
	
	log.CommitTo(5) // 超出范围
}

// TestRaftLogAppliedTo 测试应用
func TestRaftLogAppliedTo(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加并提交一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	log.CommitTo(3)
	
	// 测试正常应用
	log.AppliedTo(2)
	if log.applied != 2 {
		t.Errorf("Expected applied to be 2, got %d", log.applied)
	}
	
	// 测试应用到更高的索引
	log.AppliedTo(3)
	if log.applied != 3 {
		t.Errorf("Expected applied to be 3, got %d", log.applied)
	}
	
	// 测试应用到较低的索引（应该不变）
	log.AppliedTo(1)
	if log.applied != 3 {
		t.Errorf("Expected applied to remain 3, got %d", log.applied)
	}
}

// TestRaftLogAppliedToOutOfRange 测试应用超出范围的索引
func TestRaftLogAppliedToOutOfRange(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一个条目但不提交
	entry := LogEntry{
		Index: 1,
		Term:  1,
		Type:  EntryNormal,
		Data:  []byte("entry1"),
	}
	log.Append(entry)
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when applying beyond committed index")
		}
	}()
	
	log.AppliedTo(1) // 超出已提交的范围
}

// TestRaftLogTruncation 测试日志截断
func TestRaftLogTruncation(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	
	// 在索引 2 处追加新条目（应该截断索引 3）
	newEntry := LogEntry{
		Index: 2,
		Term:  2, // 不同的任期
		Type:  EntryNormal,
		Data:  []byte("new_entry2"),
	}
	log.Append(newEntry)
	
	// 检查最后索引
	if log.LastIndex() != 2 {
		t.Errorf("Expected last index to be 2 after truncation, got %d", log.LastIndex())
	}
	
	// 检查任期
	term, err := log.Term(2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if term != 2 {
		t.Errorf("Expected term at index 2 to be 2, got %d", term)
	}
}

// TestRaftLogUnstableEntries 测试获取未稳定的条目
func TestRaftLogUnstableEntries(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	
	// 获取未稳定的条目
	unstable := log.UnstableEntries()
	if len(unstable) != 3 {
		t.Errorf("Expected 3 unstable entries, got %d", len(unstable))
	}
	
	// 检查条目内容
	for i, entry := range unstable {
		if entry.Index != uint64(i+1) {
			t.Errorf("Expected entry %d to have index %d, got %d", i, i+1, entry.Index)
		}
	}
}

// TestRaftLogStableTo 测试稳定化条目
func TestRaftLogStableTo(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	
	// 稳定化前两个条目
	log.StableTo(2)
	
	// 检查未稳定的条目
	unstable := log.UnstableEntries()
	if len(unstable) != 1 {
		t.Errorf("Expected 1 unstable entry after stabilizing, got %d", len(unstable))
	}
	
	if unstable[0].Index != 3 {
		t.Errorf("Expected remaining unstable entry to have index 3, got %d", unstable[0].Index)
	}
}

// TestRaftLogNextEnts 测试获取下一批要应用的条目
func TestRaftLogNextEnts(t *testing.T) {
	storage := NewMemoryStorage()
	log := NewRaftLog(storage)
	
	// 添加并提交一些条目
	entries := []LogEntry{
		{Index: 1, Term: 1, Type: EntryNormal, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: EntryNormal, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: EntryNormal, Data: []byte("entry3")},
	}
	log.Append(entries...)
	log.CommitTo(3)
	
	// 获取下一批要应用的条目
	nextEnts := log.NextEnts()
	if len(nextEnts) != 3 {
		t.Errorf("Expected 3 entries to apply, got %d", len(nextEnts))
	}
	
	// 应用前两个条目
	log.AppliedTo(2)
	
	// 再次获取下一批要应用的条目
	nextEnts = log.NextEnts()
	if len(nextEnts) != 1 {
		t.Errorf("Expected 1 entry to apply after applying 2, got %d", len(nextEnts))
	}
	
	if nextEnts[0].Index != 3 {
		t.Errorf("Expected next entry to have index 3, got %d", nextEnts[0].Index)
	}
}