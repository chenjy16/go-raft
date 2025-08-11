package raft

import (
	"errors"
	"time"

	"github.com/chenjianyu/go-raft/raft/logging"
)

var (
	// ErrCompacted 表示请求的日志条目已被压缩
	ErrCompacted = errors.New("requested entry at index is unavailable due to compaction")
	// ErrSnapOutOfDate 表示快照已过期
	ErrSnapOutOfDate = errors.New("requested index is older than the existing snapshot")
	// ErrUnavailable 表示请求的日志条目不可用
	ErrUnavailable = errors.New("requested entry at index is unavailable")
)

// Storage 定义存储接口
type Storage interface {
	// InitialState 返回保存的 HardState 和 ConfState
	InitialState() (HardState, error)

	// Entries 返回指定范围的日志条目 [lo, hi)
	// MaxSize 限制返回条目的总大小
	Entries(lo, hi uint64) ([]LogEntry, error)

	// Term 返回指定索引的日志条目的任期
	Term(index uint64) (uint64, error)

	// LastIndex 返回最后一个日志条目的索引
	LastIndex() (uint64, error)

	// FirstIndex 返回第一个可用日志条目的索引
	FirstIndex() (uint64, error)

	// Snapshot 返回最新的快照
	Snapshot() (*Snapshot, error)
}

// MemoryStorage 内存存储实现
type MemoryStorage struct {
	hardState HardState
	snapshot  *Snapshot
	entries   []LogEntry

	// 事件发射器
	eventEmitter logging.EventEmitter
}

// NewMemoryStorage 创建新的内存存储
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		entries: make([]LogEntry, 1), // 索引 0 为哨兵
	}
}

// SetEventEmitter 设置事件发射器
func (ms *MemoryStorage) SetEventEmitter(emitter logging.EventEmitter) {
	ms.eventEmitter = emitter
}

// emitEvent 发射事件的便利方法
func (ms *MemoryStorage) emitEvent(event logging.Event) {
	if ms.eventEmitter != nil {
		ms.eventEmitter.EmitEvent(event)
	}
}

// InitialState 实现 Storage 接口
func (ms *MemoryStorage) InitialState() (HardState, error) {
	return ms.hardState, nil
}

// SetHardState 设置 HardState
func (ms *MemoryStorage) SetHardState(st HardState) error {
	ms.hardState = st
	return nil
}

// Entries 实现 Storage 接口
func (ms *MemoryStorage) Entries(lo, hi uint64) ([]LogEntry, error) {
	if lo > hi {
		if ms.eventEmitter != nil {
			event := &logging.BaseEvent{
				Type:      logging.EventWarning,
				Timestamp: time.Now(),
				NodeID:    0,
			}
			ms.eventEmitter.EmitEvent(event)
		}
		return nil, ErrUnavailable
	}

	if lo == hi {
		return nil, nil
	}

	if lo < ms.firstIndex() {
		return nil, ErrCompacted
	}

	if hi > ms.lastIndex()+1 {
		return nil, ErrUnavailable
	}

	if len(ms.entries) == 1 {
		return nil, nil // 没有实际的日志条目，只有哨兵
	}

	offset := ms.entries[0].Index

	if lo-offset >= uint64(len(ms.entries)) {
		return nil, ErrUnavailable
	}

	endIdx := hi - offset
	if endIdx > uint64(len(ms.entries)) {
		endIdx = uint64(len(ms.entries))
	}

	result := ms.entries[lo-offset : endIdx]
	return result, nil
}

// Term 实现 Storage 接口
func (ms *MemoryStorage) Term(index uint64) (uint64, error) {
	offset := ms.entries[0].Index

	if index < offset {
		return 0, ErrCompacted
	}

	if index-offset >= uint64(len(ms.entries)) {
		return 0, ErrUnavailable
	}

	return ms.entries[index-offset].Term, nil
}

// LastIndex 实现 Storage 接口
func (ms *MemoryStorage) LastIndex() (uint64, error) {
	return ms.lastIndex(), nil
}

// FirstIndex 实现 Storage 接口
func (ms *MemoryStorage) FirstIndex() (uint64, error) {
	return ms.firstIndex(), nil
}

// Snapshot 实现 Storage 接口
func (ms *MemoryStorage) Snapshot() (*Snapshot, error) {
	if ms.snapshot == nil {
		return &Snapshot{}, nil
	}
	return ms.snapshot, nil
}

// ApplySnapshot 应用快照
func (ms *MemoryStorage) ApplySnapshot(snapshot *Snapshot) error {
	if snapshot.Index <= ms.firstIndex() {
		return ErrSnapOutOfDate
	}

	ms.snapshot = snapshot
	ms.entries = []LogEntry{{Index: snapshot.Index, Term: snapshot.Term}}
	return nil
}

// Append 追加日志条目
func (ms *MemoryStorage) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// 发射存储追加开始事件
	if ms.eventEmitter != nil {
		event := &logging.LogAppendingEvent{
			BaseEvent:  logging.NewBaseEvent(logging.EventLogAppending, 0),
			EntryCount: len(entries),
			FirstIndex: entries[0].Index,
			LastIndex:  entries[len(entries)-1].Index,
		}
		ms.eventEmitter.EmitEvent(event)
	}

	startTime := time.Now()

	first := ms.firstIndex()
	last := entries[0].Index + uint64(len(entries)) - 1

	// 检查是否有重叠
	if last < first {
		return nil
	}

	// 截断冲突的条目
	if first > entries[0].Index {
		entries = entries[first-entries[0].Index:]
	}

	offset := entries[0].Index - ms.entries[0].Index
	if uint64(len(ms.entries)) > offset {
		ms.entries = append([]LogEntry{}, ms.entries[:offset]...)
	}

	ms.entries = append(ms.entries, entries...)

	// 发射存储追加完成事件
	if ms.eventEmitter != nil {
		event := &logging.LogAppendedEvent{
			BaseEvent:  logging.NewBaseEvent(logging.EventLogAppended, 0),
			EntryCount: len(entries),
			FirstIndex: entries[0].Index,
			LastIndex:  entries[len(entries)-1].Index,
			Duration:   time.Since(startTime),
			Error:      nil,
		}
		ms.eventEmitter.EmitEvent(event)
	}

	return nil
}

// Compact 压缩日志到指定索引
func (ms *MemoryStorage) Compact(compactIndex uint64) error {
	offset := ms.entries[0].Index
	if compactIndex <= offset {
		return ErrCompacted
	}

	if compactIndex > ms.lastIndex() {
		return ErrUnavailable
	}

	i := compactIndex - offset
	entries := make([]LogEntry, 1, 1+uint64(len(ms.entries))-i)
	entries[0].Index = ms.entries[i].Index
	entries[0].Term = ms.entries[i].Term
	entries = append(entries, ms.entries[i+1:]...)
	ms.entries = entries
	return nil
}

// CreateSnapshot 创建快照
func (ms *MemoryStorage) CreateSnapshot(index uint64, data []byte) (*Snapshot, error) {
	if index <= ms.firstIndex() {
		return nil, ErrSnapOutOfDate
	}

	if index > ms.lastIndex() {
		return nil, ErrUnavailable
	}

	offset := ms.entries[0].Index

	// 确保索引存在并获取对应的任期
	if index-offset >= uint64(len(ms.entries)) {
		return nil, ErrUnavailable
	}

	snapshot := &Snapshot{
		Index: index,
		Term:  ms.entries[index-offset].Term,
		Data:  data,
	}

	ms.snapshot = snapshot
	return snapshot, nil
}

// 内部辅助方法
func (ms *MemoryStorage) firstIndex() uint64 {
	return ms.entries[0].Index + 1
}

func (ms *MemoryStorage) lastIndex() uint64 {
	return ms.entries[0].Index + uint64(len(ms.entries)) - 1
}
