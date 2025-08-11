package raft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// FileStorage 文件存储实现
type FileStorage struct {
	mu        sync.RWMutex
	dir       string
	hardState HardState
	snapshot  *Snapshot
	entries   []LogEntry
	
	// 文件路径
	hardStatePath string
	snapshotPath  string
	entriesPath   string
}

// NewFileStorage 创建新的文件存储
func NewFileStorage(dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %v", err)
	}

	fs := &FileStorage{
		dir:           dir,
		hardStatePath: filepath.Join(dir, "hardstate.json"),
		snapshotPath:  filepath.Join(dir, "snapshot.json"),
		entriesPath:   filepath.Join(dir, "entries.json"),
		entries:       make([]LogEntry, 1), // 索引 0 为哨兵
	}

	// 加载现有数据
	if err := fs.load(); err != nil {
		return nil, fmt.Errorf("failed to load storage: %v", err)
	}

	return fs, nil
}

// load 从文件加载数据
func (fs *FileStorage) load() error {
	// 加载 HardState
	if data, err := ioutil.ReadFile(fs.hardStatePath); err == nil {
		if err := json.Unmarshal(data, &fs.hardState); err != nil {
			return fmt.Errorf("failed to unmarshal hardstate: %v", err)
		}
	}

	// 加载快照
	if data, err := ioutil.ReadFile(fs.snapshotPath); err == nil {
		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return fmt.Errorf("failed to unmarshal snapshot: %v", err)
		}
		fs.snapshot = &snapshot
	}

	// 加载日志条目
	if data, err := ioutil.ReadFile(fs.entriesPath); err == nil {
		if err := json.Unmarshal(data, &fs.entries); err != nil {
			return fmt.Errorf("failed to unmarshal entries: %v", err)
		}
	}

	return nil
}

// saveHardState 保存 HardState
func (fs *FileStorage) saveHardState() error {
	data, err := json.Marshal(fs.hardState)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fs.hardStatePath, data, 0644)
}

// saveSnapshot 保存快照
func (fs *FileStorage) saveSnapshot() error {
	if fs.snapshot == nil {
		return nil
	}
	data, err := json.Marshal(fs.snapshot)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fs.snapshotPath, data, 0644)
}

// saveEntries 保存日志条目
func (fs *FileStorage) saveEntries() error {
	data, err := json.Marshal(fs.entries)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fs.entriesPath, data, 0644)
}

// InitialState 实现 Storage 接口
func (fs *FileStorage) InitialState() (HardState, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.hardState, nil
}

// SetHardState 设置 HardState
func (fs *FileStorage) SetHardState(st HardState) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	fs.hardState = st
	return fs.saveHardState()
}

// Entries 实现 Storage 接口
func (fs *FileStorage) Entries(lo, hi uint64) ([]LogEntry, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if lo < fs.firstIndex() {
		return nil, ErrCompacted
	}

	if hi > fs.lastIndex()+1 {
		return nil, ErrUnavailable
	}

	if len(fs.entries) == 1 {
		return nil, ErrUnavailable
	}

	offset := fs.entries[0].Index
	if lo-offset >= uint64(len(fs.entries)) || hi-offset > uint64(len(fs.entries)) {
		return nil, ErrUnavailable
	}

	return fs.entries[lo-offset : hi-offset], nil
}

// Term 实现 Storage 接口
func (fs *FileStorage) Term(index uint64) (uint64, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	offset := fs.entries[0].Index

	if index < offset {
		return 0, ErrCompacted
	}

	if index-offset >= uint64(len(fs.entries)) {
		return 0, ErrUnavailable
	}

	return fs.entries[index-offset].Term, nil
}

// LastIndex 实现 Storage 接口
func (fs *FileStorage) LastIndex() (uint64, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.lastIndex(), nil
}

// FirstIndex 实现 Storage 接口
func (fs *FileStorage) FirstIndex() (uint64, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.firstIndex(), nil
}

// Snapshot 实现 Storage 接口
func (fs *FileStorage) Snapshot() (*Snapshot, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	if fs.snapshot == nil {
		return &Snapshot{}, nil
	}
	return fs.snapshot, nil
}

// Append 追加日志条目
func (fs *FileStorage) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	first := fs.firstIndex()
	last := entries[0].Index + uint64(len(entries)) - 1

	// 检查是否有重叠
	if last < first {
		return nil
	}

	// 截断冲突的条目
	if first > entries[0].Index {
		entries = entries[first-entries[0].Index:]
	}

	offset := entries[0].Index - fs.entries[0].Index
	if uint64(len(fs.entries)) > offset {
		fs.entries = append([]LogEntry{}, fs.entries[:offset]...)
	}

	fs.entries = append(fs.entries, entries...)
	return fs.saveEntries()
}

// ApplySnapshot 应用快照
func (fs *FileStorage) ApplySnapshot(snapshot *Snapshot) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if snapshot.Index <= fs.firstIndex() {
		return ErrSnapOutOfDate
	}

	fs.snapshot = snapshot
	fs.entries = []LogEntry{{Index: snapshot.Index, Term: snapshot.Term}}
	
	if err := fs.saveSnapshot(); err != nil {
		return err
	}
	return fs.saveEntries()
}

// CreateSnapshot 创建快照
func (fs *FileStorage) CreateSnapshot(index uint64, data []byte) (*Snapshot, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if index <= fs.firstIndex() {
		return nil, ErrSnapOutOfDate
	}

	if index > fs.lastIndex() {
		return nil, ErrUnavailable
	}

	offset := fs.entries[0].Index
	snapshot := &Snapshot{
		Index: index,
		Term:  fs.entries[index-offset].Term,
		Data:  data,
	}

	fs.snapshot = snapshot
	if err := fs.saveSnapshot(); err != nil {
		return nil, err
	}

	return snapshot, nil
}

// Compact 压缩日志到指定索引
func (fs *FileStorage) Compact(compactIndex uint64) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	offset := fs.entries[0].Index
	if compactIndex <= offset {
		return ErrCompacted
	}

	if compactIndex > fs.lastIndex() {
		return ErrUnavailable
	}

	i := compactIndex - offset
	entries := make([]LogEntry, 1, 1+uint64(len(fs.entries))-i)
	entries[0].Index = fs.entries[i].Index
	entries[0].Term = fs.entries[i].Term
	entries = append(entries, fs.entries[i+1:]...)
	fs.entries = entries
	
	return fs.saveEntries()
}

// 内部辅助方法
func (fs *FileStorage) firstIndex() uint64 {
	return fs.entries[0].Index + 1
}

func (fs *FileStorage) lastIndex() uint64 {
	return fs.entries[0].Index + uint64(len(fs.entries)) - 1
}

// Close 关闭存储
func (fs *FileStorage) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	// 确保所有数据都已保存
	if err := fs.saveHardState(); err != nil {
		return err
	}
	if err := fs.saveSnapshot(); err != nil {
		return err
	}
	return fs.saveEntries()
}