package raft

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chenjianyu/go-raft/raft/logging"
)

// Transport 接口定义（避免循环导入）
type Transport interface {
	Send(nodeID uint64, msg Message) error
}

// SnapshotManager 快照管理器
type SnapshotManager struct {
	mu           sync.RWMutex
	node         *Node
	storage      Storage
	stateMachine StateMachine

	// 配置
	snapshotThreshold uint64 // 触发快照的日志条目数量
	compactThreshold  uint64 // 触发压缩的日志条目数量

	// 状态
	lastSnapshotIndex uint64
	lastCompactIndex  uint64

	// 控制
	stopCh chan struct{}
	doneCh chan struct{}
	// running 标志确保 Start/Stop 幂等
	running bool

	// 事件发射器
	eventEmitter logging.EventEmitter
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager(node *Node, storage Storage, stateMachine StateMachine) *SnapshotManager {
	return &SnapshotManager{
		node:              node,
		storage:           storage,
		stateMachine:      stateMachine,
		snapshotThreshold: 1000,  // 默认 1000 条日志触发快照
		compactThreshold:  10000, // 默认 10000 条日志触发压缩
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		running:           false,
	}
}

// SetEventEmitter 设置事件发射器
func (sm *SnapshotManager) SetEventEmitter(emitter logging.EventEmitter) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventEmitter = emitter
}

// emitEvent 发射事件的便利方法
func (sm *SnapshotManager) emitEvent(event logging.Event) {
	if sm.eventEmitter != nil {
		sm.eventEmitter.EmitEvent(event)
	}
}

// SetSnapshotThreshold 设置快照阈值
func (sm *SnapshotManager) SetSnapshotThreshold(threshold uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.snapshotThreshold = threshold
}

// SetCompactThreshold 设置压缩阈值
func (sm *SnapshotManager) SetCompactThreshold(threshold uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.compactThreshold = threshold
}

// Start 启动快照管理器
func (sm *SnapshotManager) Start() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.running {
		return
	}
	// 重新创建通道，确保可以再次启动
	sm.stopCh = make(chan struct{})
	sm.doneCh = make(chan struct{})
	sm.running = true
	go sm.run()
}

// Stop 停止快照管理器
func (sm *SnapshotManager) Stop() {
	sm.mu.Lock()
	if !sm.running {
		// 已停止，直接返回
		sm.mu.Unlock()
		return
	}
	// 读取通道引用后解锁，避免阻塞期间持有锁
	stopCh := sm.stopCh
	doneCh := sm.doneCh
	sm.mu.Unlock()

	// 通知停止（只关闭一次）
	select {
	case <-stopCh:
		// 已经关闭
	default:
		close(stopCh)
	}
	<-doneCh

	sm.mu.Lock()
	sm.running = false
	sm.mu.Unlock()
}

// run 运行快照管理器
func (sm *SnapshotManager) run() {
	defer close(sm.doneCh)

	ticker := time.NewTicker(30 * time.Second) // 每 30 秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.checkAndCreateSnapshot()
			sm.checkAndCompactLog()
		case <-sm.stopCh:
			return
		}
	}
}

// checkAndCreateSnapshot 检查并创建快照
func (sm *SnapshotManager) checkAndCreateSnapshot() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	lastIndex, err := sm.storage.LastIndex()
	if err != nil {
		return
	}

	// 检查是否需要创建快照
	if lastIndex-sm.lastSnapshotIndex >= sm.snapshotThreshold {
		if err := sm.createSnapshot(lastIndex); err != nil {
			// 发射快照创建失败事件
			if sm.eventEmitter != nil {
				event := &logging.ErrorEvent{
					BaseEvent: logging.NewBaseEvent(logging.EventError, 0),
					Error:     err,
					Context:   "snapshot creation failed",
				}
				sm.eventEmitter.EmitEvent(event)
			}
		}
	}
}

// checkAndCompactLog 检查并压缩日志
func (sm *SnapshotManager) checkAndCompactLog() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	lastIndex, err := sm.storage.LastIndex()
	if err != nil {
		return
	}

	firstIndex, err := sm.storage.FirstIndex()
	if err != nil {
		return
	}

	// 检查是否需要压缩日志
	if lastIndex-firstIndex >= sm.compactThreshold {
		compactIndex := sm.lastSnapshotIndex
		if compactIndex > firstIndex {
			if err := sm.compactLog(compactIndex); err != nil {
				// 发射日志压缩失败事件
				if sm.eventEmitter != nil {
					event := &logging.ErrorEvent{
						BaseEvent: logging.NewBaseEvent(logging.EventError, 0),
						Error:     err,
						Context:   "log compaction failed",
					}
					sm.eventEmitter.EmitEvent(event)
				}
			}
		}
	}
}

// createSnapshot 创建快照
func (sm *SnapshotManager) createSnapshot(index uint64) error {
	// 获取状态机快照
	data, err := sm.stateMachine.Snapshot()
	if err != nil {
		return fmt.Errorf("failed to get state machine snapshot: %v", err)
	}

	// 创建快照
	if fs, ok := sm.storage.(*FileStorage); ok {
		snapshot, err := fs.CreateSnapshot(index, data)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %v", err)
		}

		sm.lastSnapshotIndex = snapshot.Index
		// 发射快照创建成功事件
		if sm.eventEmitter != nil {
			event := &logging.SnapshotCreatedEvent{
				BaseEvent: logging.NewBaseEvent(logging.EventSnapshotCreated, 0),
				Index:     snapshot.Index,
				Term:      snapshot.Term,
				Size:      int64(len(data)),
				Duration:  0,
				Error:     nil,
			}
			sm.eventEmitter.EmitEvent(event)
		}
	} else if ms, ok := sm.storage.(*MemoryStorage); ok {
		snapshot, err := ms.CreateSnapshot(index, data)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %v", err)
		}

		sm.lastSnapshotIndex = snapshot.Index
		// 发射快照创建成功事件
		if sm.eventEmitter != nil {
			event := &logging.SnapshotCreatedEvent{
				BaseEvent: logging.NewBaseEvent(logging.EventSnapshotCreated, 0),
				Index:     snapshot.Index,
				Term:      snapshot.Term,
				Size:      int64(len(data)),
				Duration:  0,
				Error:     nil,
			}
			sm.eventEmitter.EmitEvent(event)
		}
	}

	return nil
}

// compactLog 压缩日志
func (sm *SnapshotManager) compactLog(compactIndex uint64) error {
	if fs, ok := sm.storage.(*FileStorage); ok {
		if err := fs.Compact(compactIndex); err != nil {
			return fmt.Errorf("failed to compact log: %v", err)
		}
	} else if ms, ok := sm.storage.(*MemoryStorage); ok {
		if err := ms.Compact(compactIndex); err != nil {
			return fmt.Errorf("failed to compact log: %v", err)
		}
	}

	sm.lastCompactIndex = compactIndex

	// 发射日志压缩成功事件
	if sm.eventEmitter != nil {
		event := &logging.BaseEvent{
			Type:      logging.EventLogCommitted, // 使用现有的事件类型
			Timestamp: time.Now(),
			NodeID:    0,
		}
		sm.eventEmitter.EmitEvent(event)
	}

	return nil
}

// CreateSnapshotNow 立即创建快照
func (sm *SnapshotManager) CreateSnapshotNow() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	lastIndex, err := sm.storage.LastIndex()
	if err != nil {
		return err
	}

	return sm.createSnapshot(lastIndex)
}

// CompactLogNow 立即压缩日志
func (sm *SnapshotManager) CompactLogNow(compactIndex uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.compactLog(compactIndex)
}

// GetSnapshotInfo 获取快照信息
func (sm *SnapshotManager) GetSnapshotInfo() (uint64, uint64) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshotIndex, sm.lastCompactIndex
}

// RestoreFromSnapshot 从快照恢复
func (sm *SnapshotManager) RestoreFromSnapshot(snapshot *Snapshot) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if snapshot == nil || (snapshot.Index == 0 && snapshot.Term == 0) {
		return fmt.Errorf("invalid snapshot")
	}
	if fs, ok := sm.storage.(*FileStorage); ok {
		if err := fs.ApplySnapshot(snapshot); err != nil {
			return err
		}
	} else if ms, ok := sm.storage.(*MemoryStorage); ok {
		if err := ms.ApplySnapshot(snapshot); err != nil {
			return err
		}
	}
	sm.lastSnapshotIndex = snapshot.Index
	return nil
}

// SnapshotSender 负责发送快照
type SnapshotSender struct {
	node      *Node
	transport Transport
}

// NewSnapshotSender 创建快照发送器
func NewSnapshotSender(node *Node, transport Transport) *SnapshotSender {
	return &SnapshotSender{node: node, transport: transport}
}

// SendSnapshot 发送快照
func (ss *SnapshotSender) SendSnapshot(ctx context.Context, nodeID uint64, snapshot *Snapshot) error {
	if snapshot == nil || (snapshot.Index == 0 && snapshot.Term == 0) {
		return fmt.Errorf("invalid snapshot")
	}
	return ss.transport.Send(nodeID, Message{Type: MsgSnapshot, Snapshot: snapshot})
}

// SnapshotReceiver 接收快照
type SnapshotReceiver struct {
	node            *Node
	snapshotManager *SnapshotManager
}

// NewSnapshotReceiver 创建快照接收器
func NewSnapshotReceiver(node *Node, snapshotManager *SnapshotManager) *SnapshotReceiver {
	return &SnapshotReceiver{node: node, snapshotManager: snapshotManager}
}

// ReceiveSnapshot 接收并应用快照
func (sr *SnapshotReceiver) ReceiveSnapshot(msg Message) error {
	if msg.Snapshot == nil || (msg.Snapshot.Index == 0 && msg.Snapshot.Term == 0) {
		return fmt.Errorf("invalid snapshot")
	}
	return sr.snapshotManager.RestoreFromSnapshot(msg.Snapshot)
}
