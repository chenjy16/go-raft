package logging

import (
	"sync"
	"time"
)

// EventEmitter 事件发射器接口
type EventEmitter interface {
	// EmitEvent 发射事件
	EmitEvent(event Event)
	// SetLogger 设置日志器
	SetLogger(logger Logger)
	// GetLogger 获取日志器
	GetLogger() Logger
	// WithNodeID 设置节点ID
	WithNodeID(nodeID uint64) EventEmitter
	// Close 关闭发射器
	Close() error
}

// DefaultEventEmitter 默认事件发射器实现
type DefaultEventEmitter struct {
	logger Logger
	nodeID uint64
	mu     sync.RWMutex
}

// NewEventEmitter 创建事件发射器
func NewEventEmitter(logger Logger, nodeID uint64) *DefaultEventEmitter {
	return &DefaultEventEmitter{
		logger: logger,
		nodeID: nodeID,
	}
}

func (e *DefaultEventEmitter) EmitEvent(event Event) {
	e.mu.RLock()
	logger := e.logger
	e.mu.RUnlock()
	
	if logger != nil {
		logger.LogEvent(event)
	}
}

func (e *DefaultEventEmitter) SetLogger(logger Logger) {
	e.mu.Lock()
	e.logger = logger
	e.mu.Unlock()
}

func (e *DefaultEventEmitter) GetLogger() Logger {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.logger
}

func (e *DefaultEventEmitter) WithNodeID(nodeID uint64) EventEmitter {
	return &DefaultEventEmitter{
		logger: e.logger,
		nodeID: nodeID,
	}
}

func (e *DefaultEventEmitter) Close() error {
	e.mu.RLock()
	logger := e.logger
	e.mu.RUnlock()
	
	if logger != nil {
		return logger.Close()
	}
	return nil
}

// 便利方法：发射具体事件

func (e *DefaultEventEmitter) EmitNodeStarting(config map[string]interface{}) {
	event := &NodeStartingEvent{
		BaseEvent: NewBaseEvent(EventNodeStarting, e.nodeID),
		Config:    config,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitNodeStarted(duration time.Duration, err error) {
	event := &NodeStartedEvent{
		BaseEvent: NewBaseEvent(EventNodeStarted, e.nodeID),
		Duration:  duration,
		Error:     err,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitNodeStopping() {
	event := &BaseEvent{
		Type:      EventNodeStopping,
		Timestamp: time.Now(),
		NodeID:    e.nodeID,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitNodeStopped() {
	event := &BaseEvent{
		Type:      EventNodeStopped,
		Timestamp: time.Now(),
		NodeID:    e.nodeID,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitStateChanged(oldState, newState string) {
	event := &StateChangedEvent{
		BaseEvent: NewBaseEvent(EventStateChanged, e.nodeID),
		OldState:  oldState,
		NewState:  newState,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitTermChanged(oldTerm, newTerm uint64) {
	event := &TermChangedEvent{
		BaseEvent: NewBaseEvent(EventTermChanged, e.nodeID),
		OldTerm:   oldTerm,
		NewTerm:   newTerm,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitLeaderChanged(oldLeader, newLeader uint64) {
	event := &LeaderChangedEvent{
		BaseEvent: NewBaseEvent(EventLeaderChanged, e.nodeID),
		OldLeader: oldLeader,
		NewLeader: newLeader,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitLogAppending(entryCount int, firstIndex, lastIndex uint64) {
	event := &LogAppendingEvent{
		BaseEvent:  NewBaseEvent(EventLogAppending, e.nodeID),
		EntryCount: entryCount,
		FirstIndex: firstIndex,
		LastIndex:  lastIndex,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitLogAppended(entryCount int, firstIndex, lastIndex uint64, duration time.Duration, err error) {
	event := &LogAppendedEvent{
		BaseEvent:  NewBaseEvent(EventLogAppended, e.nodeID),
		EntryCount: entryCount,
		FirstIndex: firstIndex,
		LastIndex:  lastIndex,
		Duration:   duration,
		Error:      err,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitLogCommitting(commitIndex uint64, entryCount int) {
	event := &LogCommittingEvent{
		BaseEvent:   NewBaseEvent(EventLogCommitting, e.nodeID),
		CommitIndex: commitIndex,
		EntryCount:  entryCount,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitLogCommitted(commitIndex uint64, entryCount int, duration time.Duration) {
	event := &LogCommittedEvent{
		BaseEvent:   NewBaseEvent(EventLogCommitted, e.nodeID),
		CommitIndex: commitIndex,
		EntryCount:  entryCount,
		Duration:    duration,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitMessageSending(messageType string, to, term uint64) {
	event := &MessageSendingEvent{
		BaseEvent:   NewBaseEvent(EventMessageSending, e.nodeID),
		MessageType: messageType,
		To:          to,
		Term:        term,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitMessageSent(messageType string, to, term uint64, duration time.Duration, success bool, err error) {
	event := &MessageSentEvent{
		BaseEvent:   NewBaseEvent(EventMessageSent, e.nodeID),
		MessageType: messageType,
		To:          to,
		Term:        term,
		Duration:    duration,
		Success:     success,
		Error:       err,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitMessageReceived(messageType string, from, term uint64) {
	event := &MessageReceivedEvent{
		BaseEvent:   NewBaseEvent(EventMessageReceived, e.nodeID),
		MessageType: messageType,
		From:        from,
		Term:        term,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitElectionStarted(term uint64) {
	event := &ElectionStartedEvent{
		BaseEvent: NewBaseEvent(EventElectionStarted, e.nodeID),
		Term:      term,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitElectionWon(term uint64, voteCount int, duration time.Duration) {
	event := &ElectionWonEvent{
		BaseEvent: NewBaseEvent(EventElectionWon, e.nodeID),
		Term:      term,
		VoteCount: voteCount,
		Duration:  duration,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitVoteGranted(candidateID, term uint64) {
	event := &VoteGrantedEvent{
		BaseEvent:   NewBaseEvent(EventVoteGranted, e.nodeID),
		CandidateID: candidateID,
		Term:        term,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitSnapshotCreating(index, term uint64) {
	event := &SnapshotCreatingEvent{
		BaseEvent: NewBaseEvent(EventSnapshotCreating, e.nodeID),
		Index:     index,
		Term:      term,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitSnapshotCreated(index, term uint64, size int64, duration time.Duration, err error) {
	event := &SnapshotCreatedEvent{
		BaseEvent: NewBaseEvent(EventSnapshotCreated, e.nodeID),
		Index:     index,
		Term:      term,
		Size:      size,
		Duration:  duration,
		Error:     err,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitError(err error, context string, details map[string]interface{}) {
	event := &ErrorEvent{
		BaseEvent: NewBaseEvent(EventError, e.nodeID),
		Error:     err,
		Context:   context,
		Details:   details,
	}
	e.EmitEvent(event)
}

func (e *DefaultEventEmitter) EmitWarning(message, context string, details map[string]interface{}) {
	event := &WarningEvent{
		BaseEvent: NewBaseEvent(EventWarning, e.nodeID),
		Message:   message,
		Context:   context,
		Details:   details,
	}
	e.EmitEvent(event)
}