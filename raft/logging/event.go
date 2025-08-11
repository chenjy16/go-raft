package logging

import (
	"time"
)

// Event 定义日志事件接口 - 联合类型设计
type Event interface {
	// GetEventType 返回事件类型
	GetEventType() EventType
	// GetTimestamp 返回事件时间戳
	GetTimestamp() time.Time
	// GetNodeID 返回节点ID
	GetNodeID() uint64
}

// EventType 事件类型枚举
type EventType string

const (
	// 节点生命周期事件
	EventNodeStarting    EventType = "node_starting"
	EventNodeStarted     EventType = "node_started"
	EventNodeStopping    EventType = "node_stopping"
	EventNodeStopped     EventType = "node_stopped"
	
	// 状态变更事件
	EventStateChanged    EventType = "state_changed"
	EventTermChanged     EventType = "term_changed"
	EventLeaderChanged   EventType = "leader_changed"
	
	// 日志操作事件
	EventLogAppending    EventType = "log_appending"
	EventLogAppended     EventType = "log_appended"
	EventLogCommitting   EventType = "log_committing"
	EventLogCommitted    EventType = "log_committed"
	EventLogApplying     EventType = "log_applying"
	EventLogApplied      EventType = "log_applied"
	
	// 消息传输事件
	EventMessageSending  EventType = "message_sending"
	EventMessageSent     EventType = "message_sent"
	EventMessageReceived EventType = "message_received"
	EventMessageFailed   EventType = "message_failed"
	
	// 选举事件
	EventElectionStarted EventType = "election_started"
	EventElectionWon     EventType = "election_won"
	EventElectionLost    EventType = "election_lost"
	EventVoteGranted     EventType = "vote_granted"
	EventVoteDenied      EventType = "vote_denied"
	
	// 快照事件
	EventSnapshotCreating EventType = "snapshot_creating"
	EventSnapshotCreated  EventType = "snapshot_created"
	EventSnapshotApplying EventType = "snapshot_applying"
	EventSnapshotApplied  EventType = "snapshot_applied"
	
	// 错误事件
	EventError           EventType = "error"
	EventWarning         EventType = "warning"
)

// BaseEvent 基础事件结构
type BaseEvent struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	NodeID    uint64    `json:"node_id"`
}

func (e BaseEvent) GetEventType() EventType {
	return e.Type
}

func (e BaseEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e BaseEvent) GetNodeID() uint64 {
	return e.NodeID
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(eventType EventType, nodeID uint64) BaseEvent {
	return BaseEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		NodeID:    nodeID,
	}
}

// 具体事件类型定义

// NodeStartingEvent 节点启动事件
type NodeStartingEvent struct {
	BaseEvent
	Config map[string]interface{} `json:"config,omitempty"`
}

// NodeStartedEvent 节点启动完成事件
type NodeStartedEvent struct {
	BaseEvent
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
}

// StateChangedEvent 状态变更事件
type StateChangedEvent struct {
	BaseEvent
	OldState string `json:"old_state"`
	NewState string `json:"new_state"`
}

// TermChangedEvent 任期变更事件
type TermChangedEvent struct {
	BaseEvent
	OldTerm uint64 `json:"old_term"`
	NewTerm uint64 `json:"new_term"`
}

// LeaderChangedEvent 领导者变更事件
type LeaderChangedEvent struct {
	BaseEvent
	OldLeader uint64 `json:"old_leader"`
	NewLeader uint64 `json:"new_leader"`
}

// LogAppendingEvent 日志追加事件
type LogAppendingEvent struct {
	BaseEvent
	EntryCount int    `json:"entry_count"`
	FirstIndex uint64 `json:"first_index"`
	LastIndex  uint64 `json:"last_index"`
}

// LogAppendedEvent 日志追加完成事件
type LogAppendedEvent struct {
	BaseEvent
	EntryCount int           `json:"entry_count"`
	FirstIndex uint64        `json:"first_index"`
	LastIndex  uint64        `json:"last_index"`
	Duration   time.Duration `json:"duration"`
	Error      error         `json:"error,omitempty"`
}

// LogCommittingEvent 日志提交事件
type LogCommittingEvent struct {
	BaseEvent
	CommitIndex uint64 `json:"commit_index"`
	EntryCount  int    `json:"entry_count"`
}

// LogCommittedEvent 日志提交完成事件
type LogCommittedEvent struct {
	BaseEvent
	CommitIndex uint64        `json:"commit_index"`
	EntryCount  int           `json:"entry_count"`
	Duration    time.Duration `json:"duration"`
}

// MessageSendingEvent 消息发送事件
type MessageSendingEvent struct {
	BaseEvent
	MessageType string `json:"message_type"`
	To          uint64 `json:"to"`
	Term        uint64 `json:"term"`
}

// MessageSentEvent 消息发送完成事件
type MessageSentEvent struct {
	BaseEvent
	MessageType string        `json:"message_type"`
	To          uint64        `json:"to"`
	Term        uint64        `json:"term"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	Error       error         `json:"error,omitempty"`
}

// MessageReceivedEvent 消息接收事件
type MessageReceivedEvent struct {
	BaseEvent
	MessageType string `json:"message_type"`
	From        uint64 `json:"from"`
	Term        uint64 `json:"term"`
}

// ElectionStartedEvent 选举开始事件
type ElectionStartedEvent struct {
	BaseEvent
	Term uint64 `json:"term"`
}

// ElectionWonEvent 选举获胜事件
type ElectionWonEvent struct {
	BaseEvent
	Term      uint64        `json:"term"`
	VoteCount int           `json:"vote_count"`
	Duration  time.Duration `json:"duration"`
}

// VoteGrantedEvent 投票授予事件
type VoteGrantedEvent struct {
	BaseEvent
	CandidateID uint64 `json:"candidate_id"`
	Term        uint64 `json:"term"`
}

// SnapshotCreatingEvent 快照创建事件
type SnapshotCreatingEvent struct {
	BaseEvent
	Index uint64 `json:"index"`
	Term  uint64 `json:"term"`
}

// SnapshotCreatedEvent 快照创建完成事件
type SnapshotCreatedEvent struct {
	BaseEvent
	Index    uint64        `json:"index"`
	Term     uint64        `json:"term"`
	Size     int64         `json:"size"`
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
}

// ErrorEvent 错误事件
type ErrorEvent struct {
	BaseEvent
	Error   error  `json:"error"`
	Context string `json:"context"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// WarningEvent 警告事件
type WarningEvent struct {
	BaseEvent
	Message string `json:"message"`
	Context string `json:"context"`
	Details map[string]interface{} `json:"details,omitempty"`
}