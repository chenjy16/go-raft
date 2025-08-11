package raft

import (
	"time"
)

// NodeState 表示节点状态
type NodeState int

const (
	StateFollower NodeState = iota
	StateCandidate
	StateLeader
	StateLearner // 新增：Learner 状态
)

func (s NodeState) String() string {
	switch s {
	case StateFollower:
		return "Follower"
	case StateCandidate:
		return "Candidate"
	case StateLeader:
		return "Leader"
	case StateLearner:
		return "Learner"
	default:
		return "Unknown"
	}
}

// EntryType 日志条目类型
type EntryType int

const (
	EntryNormal EntryType = iota
	EntryConfChange
)

// ReadOnlyOption 只读选项
type ReadOnlyOption int

const (
	ReadOnlySafe ReadOnlyOption = iota // 安全读（ReadIndex）
	ReadOnlyLeaseBased                 // 基于租约的读（LeaseRead）
)

// LogEntry 表示日志条目
type LogEntry struct {
	Index uint64    `json:"index"`
	Term  uint64    `json:"term"`
	Type  EntryType `json:"type"`
	Data  []byte    `json:"data"`
}

// Message 表示节点间通信的消息
type Message struct {
	Type        MessageType `json:"type"`
	From        uint64      `json:"from"`
	To          uint64      `json:"to"`
	Term        uint64      `json:"term"`
	LogIndex    uint64      `json:"log_index"`
	LogTerm     uint64      `json:"log_term"`
	Entries     []LogEntry  `json:"entries,omitempty"`
	CommitIndex uint64      `json:"commit_index"`
	VoteGranted bool        `json:"vote_granted"`
	Success     bool        `json:"success"`
	Data        []byte      `json:"data,omitempty"`
	Snapshot    *Snapshot   `json:"snapshot,omitempty"`
	// ReadIndex 相关字段
	ReadIndex   uint64 `json:"read_index,omitempty"`
	Context     []byte `json:"context,omitempty"`
	// 网络分区处理相关
	PreVote     bool   `json:"pre_vote,omitempty"`
	Force       bool   `json:"force,omitempty"`
}

// MessageType 消息类型
type MessageType int

const (
	MsgRequestVote MessageType = iota
	MsgRequestVoteResp
	MsgAppendEntries
	MsgAppendEntriesResp
	MsgHeartbeat
	MsgHeartbeatResp
	MsgPropose
	MsgSnapshot
	MsgSnapshotResp
	MsgReadIndex        // 新增：ReadIndex 请求
	MsgReadIndexResp    // 新增：ReadIndex 响应
	MsgPreVote          // 新增：PreVote 请求（网络分区处理）
	MsgPreVoteResp      // 新增：PreVote 响应
	MsgTimeoutNow       // 新增：立即超时消息（Leader转移）
	MsgCheckQuorum      // 新增：检查法定人数
)

func (mt MessageType) String() string {
	switch mt {
	case MsgRequestVote:
		return "MsgRequestVote"
	case MsgRequestVoteResp:
		return "MsgRequestVoteResp"
	case MsgAppendEntries:
		return "MsgAppendEntries"
	case MsgAppendEntriesResp:
		return "MsgAppendEntriesResp"
	case MsgHeartbeat:
		return "MsgHeartbeat"
	case MsgHeartbeatResp:
		return "MsgHeartbeatResp"
	case MsgPropose:
		return "MsgPropose"
	case MsgSnapshot:
		return "MsgSnapshot"
	case MsgSnapshotResp:
		return "MsgSnapshotResp"
	case MsgReadIndex:
		return "MsgReadIndex"
	case MsgReadIndexResp:
		return "MsgReadIndexResp"
	case MsgPreVote:
		return "MsgPreVote"
	case MsgPreVoteResp:
		return "MsgPreVoteResp"
	case MsgTimeoutNow:
		return "MsgTimeoutNow"
	case MsgCheckQuorum:
		return "MsgCheckQuorum"
	default:
		return "Unknown"
	}
}

// Config Raft 节点配置
type Config struct {
	ID                uint64        `json:"id"`
	ElectionTimeout   time.Duration `json:"election_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ElectionTick      int           `json:"election_tick"`
	HeartbeatTick     int           `json:"heartbeat_tick"`
	Storage           Storage       `json:"-"`
	Applied           uint64        `json:"applied"`
	MaxSizePerMsg     uint64        `json:"max_size_per_msg"`
	MaxInflightMsgs   int           `json:"max_inflight_msgs"`
	MaxLogEntries     int           `json:"max_log_entries"`
	SnapshotThreshold int           `json:"snapshot_threshold"`
	// ReadIndex/LeaseRead 配置
	ReadOnlyOption    ReadOnlyOption `json:"read_only_option"`
	LeaseTimeout      time.Duration  `json:"lease_timeout"`
	// PreVote 配置（网络分区处理）
	PreVote           bool           `json:"pre_vote"`
	CheckQuorum       bool           `json:"check_quorum"`
	// Learner 配置
	IsLearner         bool           `json:"is_learner"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ID:                1,
		ElectionTimeout:   150 * time.Millisecond,
		HeartbeatInterval: 50 * time.Millisecond,
		ElectionTick:      10,
		HeartbeatTick:     1,
		MaxLogEntries:     1000,
		SnapshotThreshold: 100,
	}
}

// Snapshot 快照数据
type Snapshot struct {
	Index uint64 `json:"index"`
	Term  uint64 `json:"term"`
	Data  []byte `json:"data"`
}

// HardState 需要持久化的状态
type HardState struct {
	Term   uint64 `json:"term"`
	Vote   uint64 `json:"vote"`
	Commit uint64 `json:"commit"`
}

// SoftState 易变状态
type SoftState struct {
	Lead  uint64    `json:"lead"`
	State NodeState `json:"state"`
}

// Ready 包含需要处理的状态更新
type Ready struct {
	SoftState        *SoftState `json:"soft_state,omitempty"`
	HardState        HardState  `json:"hard_state"`
	Entries          []LogEntry `json:"entries,omitempty"`
	Snapshot         *Snapshot  `json:"snapshot,omitempty"`
	CommittedEntries []LogEntry `json:"committed_entries,omitempty"`
	Messages         []Message  `json:"messages,omitempty"`
}

// IsEmptyHardState 检查 HardState 是否为空
func IsEmptyHardState(st HardState) bool {
	return st.Term == 0 && st.Vote == 0 && st.Commit == 0
}

// IsEmptySnapshot 检查快照是否为空
func IsEmptySnapshot(snap *Snapshot) bool {
	return snap == nil || (snap.Index == 0 && snap.Term == 0)
}

// IsEmptySnap 检查快照是否为空（别名）
func IsEmptySnap(snap Snapshot) bool {
	return snap.Index == 0 && snap.Term == 0
}

// ReadState 表示读状态
type ReadState struct {
	Index      uint64 `json:"index"`
	RequestCtx []byte `json:"request_ctx"`
}

// ReadIndexStatus 表示ReadIndex请求的状态
type ReadIndexStatus struct {
	Req   Message     `json:"req"`
	Index uint64      `json:"index"`
	Acks  map[uint64]bool `json:"acks"`
}

// NodeType 节点类型
type NodeType int

const (
	NodeTypeVoter NodeType = iota
	NodeTypeLearner
)

func (nt NodeType) String() string {
	switch nt {
	case NodeTypeVoter:
		return "Voter"
	case NodeTypeLearner:
		return "Learner"
	default:
		return "Unknown"
	}
}

// NodeInfo 节点信息
type NodeInfo struct {
	ID   uint64   `json:"id"`
	Type NodeType `json:"type"`
	Addr string   `json:"addr,omitempty"`
}
