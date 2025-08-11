package raft

import (
	"context"
	"fmt"
	"sync"
)

// LearnerManager 管理Learner节点
type LearnerManager struct {
	mu       sync.RWMutex
	learners map[uint64]*LearnerInfo
}

// LearnerInfo Learner节点信息
type LearnerInfo struct {
	ID          uint64 `json:"id"`
	NextIndex   uint64 `json:"next_index"`
	MatchIndex  uint64 `json:"match_index"`
	IsActive    bool   `json:"is_active"`
	LastContact int64  `json:"last_contact"`
}

// NewLearnerManager 创建新的Learner管理器
func NewLearnerManager() *LearnerManager {
	return &LearnerManager{
		learners: make(map[uint64]*LearnerInfo),
	}
}

// AddLearner 添加Learner节点
func (lm *LearnerManager) AddLearner(id uint64, nextIndex uint64) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.learners[id] = &LearnerInfo{
		ID:         id,
		NextIndex:  nextIndex,
		MatchIndex: 0,
		IsActive:   true,
	}
}

// RemoveLearner 移除Learner节点
func (lm *LearnerManager) RemoveLearner(id uint64) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	delete(lm.learners, id)
}

// GetLearner 获取Learner信息
func (lm *LearnerManager) GetLearner(id uint64) (*LearnerInfo, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	learner, exists := lm.learners[id]
	return learner, exists
}

// GetAllLearners 获取所有Learner
func (lm *LearnerManager) GetAllLearners() map[uint64]*LearnerInfo {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[uint64]*LearnerInfo)
	for id, learner := range lm.learners {
		result[id] = &LearnerInfo{
			ID:          learner.ID,
			NextIndex:   learner.NextIndex,
			MatchIndex:  learner.MatchIndex,
			IsActive:    learner.IsActive,
			LastContact: learner.LastContact,
		}
	}
	return result
}

// UpdateLearnerProgress 更新Learner进度
func (lm *LearnerManager) UpdateLearnerProgress(id uint64, matchIndex uint64) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if learner, exists := lm.learners[id]; exists {
		learner.MatchIndex = matchIndex
		learner.NextIndex = matchIndex + 1
	}
}

// IsLearner 检查节点是否为Learner
func (lm *LearnerManager) IsLearner(id uint64) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	_, exists := lm.learners[id]
	return exists
}

// 扩展Node结构体的方法以支持Learner

// AddLearner 添加Learner节点
func (n *Node) AddLearner(ctx context.Context, id uint64) error {
	if n.state != StateLeader {
		return ErrNotLeader
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// 添加到learner管理器
	if n.learnerManager == nil {
		n.learnerManager = NewLearnerManager()
	}

	nextIndex := n.raftLog.LastIndex() + 1
	n.learnerManager.AddLearner(id, nextIndex)

	// 立即发送快照或日志条目给新的Learner
	go n.sendToLearner(id)

	return nil
}

// RemoveLearner 移除Learner节点
func (n *Node) RemoveLearner(ctx context.Context, id uint64) error {
	if n.state != StateLeader {
		return ErrNotLeader
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.learnerManager != nil {
		n.learnerManager.RemoveLearner(id)
	}

	return nil
}

// PromoteLearner 将Learner提升为Voter
func (n *Node) PromoteLearner(ctx context.Context, id uint64) error {
	if n.state != StateLeader {
		return ErrNotLeader
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// 检查是否为Learner
	if n.learnerManager == nil || !n.learnerManager.IsLearner(id) {
		return fmt.Errorf("node %d is not a learner", id)
	}

	// 获取Learner信息
	learner, _ := n.learnerManager.GetLearner(id)

	// 检查Learner是否已经同步
	if learner.MatchIndex < n.raftLog.LastIndex() {
		return fmt.Errorf("learner %d is not caught up", id)
	}

	// 移除Learner身份
	n.learnerManager.RemoveLearner(id)

	// 添加为Voter
	n.peers[id] = true
	n.nextIndex[id] = learner.NextIndex
	n.matchIndex[id] = learner.MatchIndex

	return nil
}

// sendToLearner 向Learner发送日志条目
func (n *Node) sendToLearner(learnerID uint64) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.learnerManager == nil {
		return
	}

	learner, exists := n.learnerManager.GetLearner(learnerID)
	if !exists {
		return
	}

	// 检查是否需要发送快照
	firstIndex, err := n.raftLog.storage.FirstIndex()
	if err != nil {
		return
	}
	if learner.NextIndex <= firstIndex {
		n.sendSnapshotToLearner(learnerID)
		return
	}

	// 发送日志条目
	n.sendAppendEntriesToLearner(learnerID, learner)
}

// sendSnapshotToLearner 向Learner发送快照
func (n *Node) sendSnapshotToLearner(learnerID uint64) {
	snapshot, err := n.raftLog.Snapshot()
	if err != nil {
		return
	}

	msg := Message{
		Type:     MsgSnapshot,
		From:     n.id,
		To:       learnerID,
		Term:     n.currentTerm,
		Snapshot: snapshot,
	}

	n.send(msg)
}

// sendAppendEntriesToLearner 向Learner发送日志条目
func (n *Node) sendAppendEntriesToLearner(learnerID uint64, learner *LearnerInfo) {
	prevLogIndex := learner.NextIndex - 1
	prevLogTerm, err := n.raftLog.Term(prevLogIndex)
	if err != nil {
		// 如果无法获取term，发送快照
		n.sendSnapshotToLearner(learnerID)
		return
	}

	// 获取要发送的日志条目
	entries, err := n.raftLog.Entries(learner.NextIndex, n.raftLog.LastIndex()+1)
	if err != nil {
		return
	}

	msg := Message{
		Type:        MsgAppendEntries,
		From:        n.id,
		To:          learnerID,
		Term:        n.currentTerm,
		LogIndex:    prevLogIndex,
		LogTerm:     prevLogTerm,
		Entries:     entries,
		CommitIndex: n.commitIndex,
	}

	n.send(msg)
}

// handleLearnerResponse 处理Learner的响应
func (n *Node) handleLearnerResponse(msg Message) {
	if n.learnerManager == nil {
		return
	}

	learner, exists := n.learnerManager.GetLearner(msg.From)
	if !exists {
		return
	}

	if msg.Success {
		// 更新Learner进度
		if len(msg.Entries) > 0 {
			lastIndex := msg.Entries[len(msg.Entries)-1].Index
			n.learnerManager.UpdateLearnerProgress(msg.From, lastIndex)
		}
	} else {
		// 回退NextIndex
		if learner.NextIndex > 1 {
			learner.NextIndex--
		}
		// 重新发送
		go n.sendToLearner(msg.From)
	}
}

// IsLearnerNode 检查当前节点是否为Learner
func (n *Node) IsLearnerNode() bool {
	return n.config.IsLearner
}

// GetLearnerStatus 获取所有Learner状态
func (n *Node) GetLearnerStatus() map[uint64]*LearnerInfo {
	if n.learnerManager == nil {
		return make(map[uint64]*LearnerInfo)
	}
	return n.learnerManager.GetAllLearners()
}