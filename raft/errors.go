package raft

import "errors"

// 定义 Raft 相关的错误
var (
	ErrNotLeader        = errors.New("not leader")
	ErrStopped          = errors.New("raft stopped")
	ErrTimeout          = errors.New("request timeout")
	ErrProposalDropped  = errors.New("proposal dropped")
	ErrStepLocalMsg     = errors.New("cannot step local message")
	ErrStepPeerNotFound = errors.New("peer not found")
	ErrSnapshotTemporarilyUnavailable = errors.New("snapshot temporarily unavailable")
)