package raft

import (
	"fmt"
	"time"
	
	"github.com/chenjianyu/go-raft/raft/logging"
)

// RaftLog 管理 Raft 日志
type RaftLog struct {
	// 存储接口
	storage Storage

	// 未持久化的日志条目
	unstable []LogEntry

	// 已提交的日志索引
	committed uint64

	// 已应用的日志索引
	applied uint64

	// 日志的起始索引（用于快照后的日志压缩）
	offset uint64
	
	// 事件发射器
	eventEmitter logging.EventEmitter
}

// NewRaftLog 创建新的 RaftLog
func NewRaftLog(storage Storage) *RaftLog {
	if storage == nil {
		panic("storage cannot be nil")
	}

	log := &RaftLog{
		storage:   storage,
		unstable:  make([]LogEntry, 0),
		committed: 0,
		applied:   0,
		offset:    0,
	}

	// 从存储中恢复状态
	firstIndex, err := storage.FirstIndex()
	if err != nil {
		panic(err)
	}
	lastIndex, err := storage.LastIndex()
	if err != nil {
		panic(err)
	}

	log.offset = firstIndex
	log.committed = lastIndex
	log.applied = firstIndex - 1

	return log
}

// SetEventEmitter 设置事件发射器
func (l *RaftLog) SetEventEmitter(emitter logging.EventEmitter) {
	l.eventEmitter = emitter
}

// emitEvent 发射事件的便利方法
func (l *RaftLog) emitEvent(event logging.Event) {
	if l.eventEmitter != nil {
		l.eventEmitter.EmitEvent(event)
	}
}

// LastIndex 返回最后一个日志条目的索引
func (l *RaftLog) LastIndex() uint64 {
	if len(l.unstable) > 0 {
		return l.unstable[len(l.unstable)-1].Index
	}

	lastIndex, err := l.storage.LastIndex()
	if err != nil {
		return 0
	}
	return lastIndex
}

// LastTerm 返回最后一个日志条目的任期
func (l *RaftLog) LastTerm() uint64 {
	index := l.LastIndex()
	term, err := l.Term(index)
	if err != nil {
		return 0
	}
	return term
}

// Term 返回指定索引的日志条目的任期
func (l *RaftLog) Term(index uint64) (uint64, error) {
	if index == 0 {
		return 0, nil
	}

	// 检查是否在未持久化的日志中
	if len(l.unstable) > 0 {
		first := l.unstable[0].Index
		if index >= first {
			last := l.unstable[len(l.unstable)-1].Index
			if index <= last {
				return l.unstable[index-first].Term, nil
			}
			// 超出未持久化日志范围的索引不可用（尚未持久化到存储中）
			return 0, ErrUnavailable
		}
	}

	// 从存储中获取
	return l.storage.Term(index)
}

// Entries 返回指定范围的日志条目
func (l *RaftLog) Entries(lo, hi uint64) ([]LogEntry, error) {
	if lo > hi {
		return nil, fmt.Errorf("invalid range [%d, %d)", lo, hi)
	}

	if lo == hi {
		return nil, nil
	}

	lastIndex := l.LastIndex()
	if lo > lastIndex {
		// 发射日志获取事件
		if l.eventEmitter != nil {
			event := &logging.BaseEvent{
				Type:      logging.EventWarning,
				Timestamp: time.Now(),
				NodeID:    0, // 在 RaftLog 层面没有节点ID
			}
			l.eventEmitter.EmitEvent(event)
		}
		return nil, ErrUnavailable
	}

	var entries []LogEntry

	// 先确定需要从存储中获取的范围
	unstableFirstIndex := uint64(1<<63 - 1) // 最大值，表示没有未持久化条目
	if len(l.unstable) > 0 {
		unstableFirstIndex = l.unstable[0].Index
	}

	// 从存储中获取
	if lo < unstableFirstIndex {
		storedHi := min(hi, unstableFirstIndex)
		storedEntries, err := l.storage.Entries(lo, storedHi)
		if err != nil && err != ErrUnavailable {
			return nil, err
		}
		entries = append(entries, storedEntries...)
	}

	// 从未持久化日志中获取
	if len(l.unstable) > 0 && hi > unstableFirstIndex {
		unstableStart := max(lo, unstableFirstIndex)
		unstableStartOffset := unstableStart - unstableFirstIndex
		unstableEndOffset := min(hi, l.unstable[len(l.unstable)-1].Index+1) - unstableFirstIndex
		
		if unstableStartOffset < uint64(len(l.unstable)) {
			unstableEntries := l.unstable[unstableStartOffset:min(unstableEndOffset, uint64(len(l.unstable)))]
			entries = append(entries, unstableEntries...)
		}
	}

	return entries, nil
}

// Append 追加日志条目
func (l *RaftLog) Append(entries ...LogEntry) {
	if len(entries) == 0 {
		return
	}

	// 发射日志追加开始事件
	if l.eventEmitter != nil {
		event := &logging.LogAppendingEvent{
			BaseEvent:  logging.NewBaseEvent(logging.EventLogAppending, 0),
			EntryCount: len(entries),
			FirstIndex: entries[0].Index,
			LastIndex:  entries[len(entries)-1].Index,
		}
		l.eventEmitter.EmitEvent(event)
	}

	startTime := time.Now()

	// 检查是否需要截断现有的未持久化日志
	after := entries[0].Index - 1
	if after < l.offset+uint64(len(l.unstable)) {
		// 需要截断
		if after >= l.offset {
			l.unstable = l.unstable[:after-l.offset]
		} else {
			l.unstable = l.unstable[:0]
		}
	}

	l.unstable = append(l.unstable, entries...)

	// 发射日志追加完成事件
	if l.eventEmitter != nil {
		event := &logging.LogAppendedEvent{
			BaseEvent:  logging.NewBaseEvent(logging.EventLogAppended, 0),
			EntryCount: len(entries),
			FirstIndex: entries[0].Index,
			LastIndex:  entries[len(entries)-1].Index,
			Duration:   time.Since(startTime),
			Error:      nil,
		}
		l.eventEmitter.EmitEvent(event)
	}
}

// CommitTo 提交到指定索引
func (l *RaftLog) CommitTo(index uint64) {
	if index > l.LastIndex() {
		panic(fmt.Sprintf("commit index %d is out of range [%d, %d]", index, l.committed, l.LastIndex()))
	}

	if index > l.committed {
		l.committed = index
	}
}

// AppliedTo 应用到指定索引
func (l *RaftLog) AppliedTo(index uint64) {
	if index > l.committed {
		panic(fmt.Sprintf("applied index %d is out of range [%d, %d]", index, l.applied, l.committed))
	}

	if index > l.applied {
		l.applied = index
	}
}

// UnstableEntries 返回未持久化的日志条目
func (l *RaftLog) UnstableEntries() []LogEntry {
	if len(l.unstable) == 0 {
		return nil
	}
	return append([]LogEntry(nil), l.unstable...)
}

// NextEnts 返回可以应用的日志条目
func (l *RaftLog) NextEnts() []LogEntry {
	if l.applied >= l.committed {
		return nil
	}

	entries, err := l.Entries(l.applied+1, l.committed+1)
	if err != nil {
		// 发射错误事件
		if l.eventEmitter != nil {
			event := &logging.BaseEvent{
				Type:      logging.EventError,
				Timestamp: time.Now(),
				NodeID:    0,
			}
			l.eventEmitter.EmitEvent(event)
		}
		panic(err)
	}
	return entries
}

// HasNextEnts 检查是否有可应用的日志条目
func (l *RaftLog) HasNextEnts() bool {
	return l.applied < l.committed
}

// Snapshot 返回快照
func (l *RaftLog) Snapshot() (*Snapshot, error) {
	return l.storage.Snapshot()
}

// IsUpToDate 检查给定的日志是否比当前日志更新
func (l *RaftLog) IsUpToDate(index, term uint64) bool {
	lastTerm := l.LastTerm()
	return term > lastTerm || (term == lastTerm && index >= l.LastIndex())
}

// MatchTerm 检查给定索引和任期是否匹配
func (l *RaftLog) MatchTerm(index, term uint64) bool {
	t, err := l.Term(index)
	if err != nil {
		return false
	}
	return t == term
}

// MaybeCommit 尝试提交到指定索引
func (l *RaftLog) MaybeCommit(index, term uint64) bool {
	if index > l.committed && l.MatchTerm(index, term) {
		l.CommitTo(index)
		return true
	}
	return false
}

// Restore 从快照恢复
func (l *RaftLog) Restore(snapshot *Snapshot) {
	l.committed = snapshot.Index
	l.applied = snapshot.Index
	l.offset = snapshot.Index + 1
	l.unstable = l.unstable[:0]
}

// StableTo 标记日志条目为已持久化
func (l *RaftLog) StableTo(index uint64) {
	if len(l.unstable) == 0 {
		return
	}

	if index < l.unstable[0].Index {
		return
	}

	if index >= l.unstable[len(l.unstable)-1].Index {
		l.unstable = l.unstable[:0]
	} else {
		n := index - l.unstable[0].Index + 1
		l.unstable = l.unstable[n:]
	}
}

// 辅助函数
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
