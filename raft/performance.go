package raft

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PerformanceOptimizer 性能优化器
type PerformanceOptimizer struct {
	mu   sync.RWMutex
	node *Node

	// 批处理配置
	batchConfig BatchConfig

	// 流水线配置
	pipelineConfig PipelineConfig

	// 预投票配置
	preVoteEnabled bool

	// 性能统计
	stats *PerformanceStats

	// 批处理缓冲区
	batchBuffer [][]byte
	batchTimer  *time.Timer

	// 流水线状态
	pipelineState *PipelineState
}

// BatchConfig 批处理配置
type BatchConfig struct {
	Enabled  bool          `json:"enabled"`
	MaxSize  int           `json:"max_size"`
	MaxDelay time.Duration `json:"max_delay"`
	MaxBytes int           `json:"max_bytes"`
}

// PipelineConfig 流水线配置
type PipelineConfig struct {
	Enabled        bool `json:"enabled"`
	MaxInflight    int  `json:"max_inflight"`
	WindowSize     int  `json:"window_size"`
	EnableParallel bool `json:"enable_parallel"`
}

// PipelineState 流水线状态
type PipelineState struct {
	inflightCount int
	windowStart   uint64
	windowEnd     uint64
	pendingAcks   map[uint64]bool
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	mu sync.RWMutex

	// 吞吐量统计
	TotalProposals   uint64 `json:"total_proposals"`
	TotalCommits     uint64 `json:"total_commits"`
	BatchedProposals uint64 `json:"batched_proposals"`

	// 延迟统计
	AvgLatency time.Duration `json:"avg_latency"`
	P99Latency time.Duration `json:"p99_latency"`

	// 网络统计
	MessagesSent     uint64 `json:"messages_sent"`
	MessagesReceived uint64 `json:"messages_received"`
	BytesSent        uint64 `json:"bytes_sent"`
	BytesReceived    uint64 `json:"bytes_received"`

	// 选举统计
	ElectionCount   uint64        `json:"election_count"`
	AvgElectionTime time.Duration `json:"avg_election_time"`

	// 快照统计
	SnapshotCount   uint64        `json:"snapshot_count"`
	AvgSnapshotTime time.Duration `json:"avg_snapshot_time"`

	// 时间戳
	StartTime      time.Time `json:"start_time"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

// NewPerformanceOptimizer 创建性能优化器
func NewPerformanceOptimizer(node *Node) *PerformanceOptimizer {
	return &PerformanceOptimizer{
		node: node,
		batchConfig: BatchConfig{
			Enabled:  true,
			MaxSize:  100,
			MaxDelay: 10 * time.Millisecond,
			MaxBytes: 1024 * 1024, // 1MB
		},
		pipelineConfig: PipelineConfig{
			Enabled:        true,
			MaxInflight:    64,
			WindowSize:     1000,
			EnableParallel: true,
		},
		preVoteEnabled: true,
		stats: &PerformanceStats{
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
		batchBuffer: make([][]byte, 0),
		pipelineState: &PipelineState{
			pendingAcks: make(map[uint64]bool),
		},
	}
}

// EnableBatching 启用批处理
func (po *PerformanceOptimizer) EnableBatching(config BatchConfig) {
	po.mu.Lock()
	defer po.mu.Unlock()

	po.batchConfig = config
	if config.Enabled && po.batchTimer == nil {
		po.batchTimer = time.NewTimer(config.MaxDelay)
		go po.batchProcessor()
	}
}

// EnablePipelining 启用流水线
func (po *PerformanceOptimizer) EnablePipelining(config PipelineConfig) {
	po.mu.Lock()
	defer po.mu.Unlock()

	po.pipelineConfig = config
}

// EnablePreVote 启用预投票
func (po *PerformanceOptimizer) EnablePreVote(enabled bool) {
	po.mu.Lock()
	defer po.mu.Unlock()

	po.preVoteEnabled = enabled
}

// BatchPropose 批量提议
func (po *PerformanceOptimizer) BatchPropose(ctx context.Context, data []byte) error {
	if !po.batchConfig.Enabled {
		return po.node.Propose(ctx, data)
	}

	po.mu.Lock()
	defer po.mu.Unlock()

	// 添加到批处理缓冲区
	po.batchBuffer = append(po.batchBuffer, data)

	// 检查是否需要立即发送
	if len(po.batchBuffer) >= po.batchConfig.MaxSize ||
		po.calculateBatchSize() >= po.batchConfig.MaxBytes {
		return po.flushBatch(ctx)
	}

	// 重置定时器
	if po.batchTimer != nil {
		po.batchTimer.Reset(po.batchConfig.MaxDelay)
	}

	return nil
}

// flushBatch 刷新批处理缓冲区
func (po *PerformanceOptimizer) flushBatch(ctx context.Context) error {
	if len(po.batchBuffer) == 0 {
		return nil
	}

	// 创建批处理数据
	batchData := po.encodeBatch(po.batchBuffer)

	// 更新统计
	po.stats.mu.Lock()
	po.stats.BatchedProposals += uint64(len(po.batchBuffer))
	po.stats.mu.Unlock()

	// 清空缓冲区
	po.batchBuffer = po.batchBuffer[:0]

	// 提议批处理数据
	return po.node.Propose(ctx, batchData)
}

// batchProcessor 批处理处理器
func (po *PerformanceOptimizer) batchProcessor() {
	for {
		select {
		case <-po.batchTimer.C:
			po.mu.Lock()
			if len(po.batchBuffer) > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				po.flushBatch(ctx)
				cancel()
			}
			po.batchTimer.Reset(po.batchConfig.MaxDelay)
			po.mu.Unlock()
		}
	}
}

// calculateBatchSize 计算批处理大小
func (po *PerformanceOptimizer) calculateBatchSize() int {
	size := 0
	for _, data := range po.batchBuffer {
		size += len(data)
	}
	return size
}

// encodeBatch 编码批处理数据
func (po *PerformanceOptimizer) encodeBatch(batch [][]byte) []byte {
	// 简单的编码方式：长度前缀 + 数据
	result := make([]byte, 0)

	// 添加批处理标记
	result = append(result, 0xFF, 0xFE) // 批处理魔数

	// 添加条目数量
	count := uint32(len(batch))
	result = append(result,
		byte(count>>24), byte(count>>16),
		byte(count>>8), byte(count))

	// 添加每个条目
	for _, data := range batch {
		length := uint32(len(data))
		result = append(result,
			byte(length>>24), byte(length>>16),
			byte(length>>8), byte(length))
		result = append(result, data...)
	}

	return result
}

// decodeBatch 解码批处理数据
func (po *PerformanceOptimizer) decodeBatch(data []byte) ([][]byte, bool) {
	if len(data) < 6 {
		return nil, false
	}

	// 检查批处理标记
	if data[0] != 0xFF || data[1] != 0xFE {
		return nil, false
	}

	// 读取条目数量
	count := uint32(data[2])<<24 | uint32(data[3])<<16 |
		uint32(data[4])<<8 | uint32(data[5])

	result := make([][]byte, 0, count)
	offset := 6

	for i := uint32(0); i < count; i++ {
		if offset+4 > len(data) {
			return nil, false
		}

		// 读取长度
		length := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
			uint32(data[offset+2])<<8 | uint32(data[offset+3])
		offset += 4

		if offset+int(length) > len(data) {
			return nil, false
		}

		// 读取数据
		item := make([]byte, length)
		copy(item, data[offset:offset+int(length)])
		result = append(result, item)
		offset += int(length)
	}

	return result, true
}

// PipelinedAppendEntries 流水线式追加条目
func (po *PerformanceOptimizer) PipelinedAppendEntries(nodeID uint64, entries []LogEntry) error {
	if !po.pipelineConfig.Enabled {
		// 使用传统方式
		return po.sendAppendEntries(nodeID, entries)
	}

	po.mu.Lock()
	defer po.mu.Unlock()

	// 检查流水线窗口
	if po.pipelineState.inflightCount >= po.pipelineConfig.MaxInflight {
		return fmt.Errorf("pipeline window full")
	}

	// 发送条目
	if err := po.sendAppendEntries(nodeID, entries); err != nil {
		return err
	}

	// 更新流水线状态
	po.pipelineState.inflightCount++
	if len(entries) > 0 {
		lastIndex := entries[len(entries)-1].Index
		po.pipelineState.pendingAcks[lastIndex] = true
	}

	return nil
}

// sendAppendEntries 发送追加条目消息
func (po *PerformanceOptimizer) sendAppendEntries(nodeID uint64, entries []LogEntry) error {
	// 这里应该调用实际的发送逻辑
	// 简化实现
	return nil
}

// HandleAppendEntriesResponse 处理追加条目响应
func (po *PerformanceOptimizer) HandleAppendEntriesResponse(nodeID uint64, success bool, matchIndex uint64) {
	if !po.pipelineConfig.Enabled {
		return
	}

	po.mu.Lock()
	defer po.mu.Unlock()

	if success {
		// 移除已确认的条目
		delete(po.pipelineState.pendingAcks, matchIndex)
		po.pipelineState.inflightCount--

		// 更新窗口
		if matchIndex > po.pipelineState.windowStart {
			po.pipelineState.windowStart = matchIndex
		}
	} else {
		// 处理失败情况
		po.pipelineState.inflightCount--
	}
}

// PreVote 预投票
func (po *PerformanceOptimizer) PreVote(ctx context.Context) bool {
	if !po.preVoteEnabled {
		return true // 跳过预投票
	}

	// 实现预投票逻辑
	// 这里简化实现
	return true
}

// UpdateStats 更新性能统计
func (po *PerformanceOptimizer) UpdateStats(statType string, value interface{}) {
	po.stats.mu.Lock()
	defer po.stats.mu.Unlock()

	po.stats.LastUpdateTime = time.Now()

	switch statType {
	case "proposal":
		po.stats.TotalProposals++
	case "commit":
		po.stats.TotalCommits++
	case "message_sent":
		po.stats.MessagesSent++
		if bytes, ok := value.(uint64); ok {
			po.stats.BytesSent += bytes
		}
	case "message_received":
		po.stats.MessagesReceived++
		if bytes, ok := value.(uint64); ok {
			po.stats.BytesReceived += bytes
		}
	case "election":
		po.stats.ElectionCount++
		if duration, ok := value.(time.Duration); ok {
			// 计算平均选举时间
			if po.stats.ElectionCount == 1 {
				po.stats.AvgElectionTime = duration
			} else {
				po.stats.AvgElectionTime = (po.stats.AvgElectionTime*time.Duration(po.stats.ElectionCount-1) + duration) / time.Duration(po.stats.ElectionCount)
			}
		}
	case "snapshot":
		po.stats.SnapshotCount++
		if duration, ok := value.(time.Duration); ok {
			// 计算平均快照时间
			if po.stats.SnapshotCount == 1 {
				po.stats.AvgSnapshotTime = duration
			} else {
				po.stats.AvgSnapshotTime = (po.stats.AvgSnapshotTime*time.Duration(po.stats.SnapshotCount-1) + duration) / time.Duration(po.stats.SnapshotCount)
			}
		}
	}
}

// GetStats 获取性能统计
func (po *PerformanceOptimizer) GetStats() *PerformanceStats {
	po.stats.mu.RLock()
	defer po.stats.mu.RUnlock()

	// 返回副本，避免拷贝锁
	return &PerformanceStats{
		StartTime:        po.stats.StartTime,
		LastUpdateTime:   po.stats.LastUpdateTime,
		TotalProposals:   po.stats.TotalProposals,
		TotalCommits:     po.stats.TotalCommits,
		BatchedProposals: po.stats.BatchedProposals,
		MessagesSent:     po.stats.MessagesSent,
		MessagesReceived: po.stats.MessagesReceived,
		BytesSent:        po.stats.BytesSent,
		BytesReceived:    po.stats.BytesReceived,
		ElectionCount:    po.stats.ElectionCount,
		SnapshotCount:    po.stats.SnapshotCount,
		AvgLatency:       po.stats.AvgLatency,
		AvgElectionTime:  po.stats.AvgElectionTime,
		AvgSnapshotTime:  po.stats.AvgSnapshotTime,
	}
}

// GetThroughput 获取吞吐量（每秒提议数）
func (po *PerformanceOptimizer) GetThroughput() float64 {
	stats := po.GetStats()
	duration := time.Since(stats.StartTime).Seconds()
	if duration == 0 {
		return 0
	}
	return float64(stats.TotalProposals) / duration
}

// GetCommitRate 获取提交率
func (po *PerformanceOptimizer) GetCommitRate() float64 {
	stats := po.GetStats()
	if stats.TotalProposals == 0 {
		return 0
	}
	return float64(stats.TotalCommits) / float64(stats.TotalProposals)
}

// GetBatchEfficiency 获取批处理效率
func (po *PerformanceOptimizer) GetBatchEfficiency() float64 {
	stats := po.GetStats()
	if stats.TotalProposals == 0 {
		return 0
	}
	return float64(stats.BatchedProposals) / float64(stats.TotalProposals)
}

// OptimizeConfiguration 自动优化配置
func (po *PerformanceOptimizer) OptimizeConfiguration() {
	stats := po.GetStats()

	// 根据统计信息调整配置
	throughput := po.GetThroughput()
	commitRate := po.GetCommitRate()

	po.mu.Lock()
	defer po.mu.Unlock()

	// 调整批处理配置
	if throughput > 1000 && commitRate > 0.9 {
		// 高吞吐量，增加批处理大小
		if po.batchConfig.MaxSize < 200 {
			po.batchConfig.MaxSize += 10
		}
	} else if throughput < 100 || commitRate < 0.7 {
		// 低吞吐量或低提交率，减少批处理大小
		if po.batchConfig.MaxSize > 10 {
			po.batchConfig.MaxSize -= 5
		}
	}

	// 调整流水线配置
	if stats.AvgLatency > 100*time.Millisecond {
		// 高延迟，减少流水线深度
		if po.pipelineConfig.MaxInflight > 10 {
			po.pipelineConfig.MaxInflight -= 5
		}
	} else if stats.AvgLatency < 10*time.Millisecond {
		// 低延迟，增加流水线深度
		if po.pipelineConfig.MaxInflight < 100 {
			po.pipelineConfig.MaxInflight += 5
		}
	}
}

// ResetStats 重置统计信息
func (po *PerformanceOptimizer) ResetStats() {
	po.stats.mu.Lock()
	defer po.stats.mu.Unlock()

	po.stats = &PerformanceStats{
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
	}
}
