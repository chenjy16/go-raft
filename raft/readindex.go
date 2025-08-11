package raft

import (
	"context"
	"sync"
	"time"
)

// ReadIndexManager 管理ReadIndex请求
type ReadIndexManager struct {
	mu           sync.RWMutex
	pendingReads map[string]*ReadIndexStatus
	readStates   []ReadState
	option       ReadOnlyOption
	leaseTimeout time.Duration
	lastLeaseTime time.Time
}

// NewReadIndexManager 创建新的ReadIndex管理器
func NewReadIndexManager(option ReadOnlyOption, leaseTimeout time.Duration) *ReadIndexManager {
	return &ReadIndexManager{
		pendingReads:  make(map[string]*ReadIndexStatus),
		readStates:    make([]ReadState, 0),
		option:        option,
		leaseTimeout:  leaseTimeout,
		lastLeaseTime: time.Now(),
	}
}

// AddRequest 添加ReadIndex请求
func (rim *ReadIndexManager) AddRequest(req Message) {
	rim.mu.Lock()
	defer rim.mu.Unlock()

	ctx := string(req.Context)
	rim.pendingReads[ctx] = &ReadIndexStatus{
		Req:   req,
		Index: 0,
		Acks:  make(map[uint64]bool),
	}
}

// RecvAck 接收ReadIndex确认
func (rim *ReadIndexManager) RecvAck(from uint64, ctx []byte) {
	rim.mu.Lock()
	defer rim.mu.Unlock()

	ctxStr := string(ctx)
	if status, ok := rim.pendingReads[ctxStr]; ok {
		status.Acks[from] = true
	}
}

// Advance 推进ReadIndex状态
func (rim *ReadIndexManager) Advance(commitIndex uint64, quorum int) []ReadState {
	rim.mu.Lock()
	defer rim.mu.Unlock()

	var readyStates []ReadState

	for ctx, status := range rim.pendingReads {
		if len(status.Acks) >= quorum-1 { // 减1因为不包括leader自己
			readyStates = append(readyStates, ReadState{
				Index:      commitIndex,
				RequestCtx: []byte(ctx),
			})
			delete(rim.pendingReads, ctx)
		}
	}

	return readyStates
}

// CanRead 检查是否可以进行租约读
func (rim *ReadIndexManager) CanRead() bool {
	if rim.option != ReadOnlyLeaseBased {
		return false
	}

	rim.mu.RLock()
	defer rim.mu.RUnlock()

	return time.Since(rim.lastLeaseTime) < rim.leaseTimeout
}

// RenewLease 续约
func (rim *ReadIndexManager) RenewLease() {
	rim.mu.Lock()
	defer rim.mu.Unlock()

	rim.lastLeaseTime = time.Now()
}

// GetReadStates 获取就绪的读状态
func (rim *ReadIndexManager) GetReadStates() []ReadState {
	rim.mu.Lock()
	defer rim.mu.Unlock()

	states := rim.readStates
	rim.readStates = rim.readStates[:0]
	return states
}

// ReadIndexRequest 表示ReadIndex请求
type ReadIndexRequest struct {
	Context context.Context
	Done    chan ReadIndexResponse
}

// ReadIndexResponse 表示ReadIndex响应
type ReadIndexResponse struct {
	Index uint64
	Error error
}

// LinearizableRead 执行线性化读
func (n *Node) LinearizableRead(ctx context.Context) (uint64, error) {
	if n.state == StateLeader {
		switch n.config.ReadOnlyOption {
		case ReadOnlyLeaseBased:
			// LeaseRead: 如果租约有效，直接返回当前commitIndex
			if n.readIndexManager.CanRead() {
				return n.commitIndex, nil
			}
			// 租约过期，降级到ReadIndex
			fallthrough
		case ReadOnlySafe:
			// ReadIndex: 需要确认leader身份
			return n.performReadIndex(ctx)
		}
	}

	// 非leader节点，转发给leader
	return 0, ErrNotLeader
}

// performReadIndex 执行ReadIndex流程
func (n *Node) performReadIndex(ctx context.Context) (uint64, error) {
	// 生成唯一的context
	reqCtx := generateReadContext()

	// 创建ReadIndex消息
	msg := Message{
		Type:    MsgReadIndex,
		From:    n.id,
		Context: reqCtx,
	}

	// 添加到pending requests
	n.readIndexManager.AddRequest(msg)

	// 发送心跳给所有follower以确认leader身份
	n.broadcastHeartbeat()

	// 等待足够的确认
	done := make(chan uint64, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				states := n.readIndexManager.Advance(n.commitIndex, n.quorum())
				for _, state := range states {
					if string(state.RequestCtx) == string(reqCtx) {
						done <- state.Index
						return
					}
				}
			}
		}
	}()

	select {
	case index := <-done:
		return index, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// generateReadContext 生成读请求的唯一上下文
func generateReadContext() []byte {
	// 简单实现：使用时间戳
	return []byte(time.Now().Format("20060102150405.000000"))
}

// broadcastHeartbeat 广播心跳以确认leader身份
func (n *Node) broadcastHeartbeat() {
	for id := range n.peers {
		if id != n.id {
			msg := Message{
				Type:        MsgHeartbeat,
				From:        n.id,
				To:          id,
				Term:        n.currentTerm,
				CommitIndex: n.commitIndex,
			}
			n.send(msg)
		}
	}
}