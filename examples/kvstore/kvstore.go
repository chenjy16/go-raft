package kvstore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// KVStateMachine 键值存储状态机
type KVStateMachine struct {
	mu   sync.RWMutex
	data map[string]string

	// 统计信息
	stats *KVStats

	// 元数据
	metadata *KVMetadata
}

// NewKVStateMachine 创建新的键值存储状态机
func NewKVStateMachine() *KVStateMachine {
	return &KVStateMachine{
		data:     make(map[string]string),
		stats:    newKVStats(),
		metadata: newKVMetadata(),
	}
}

// Command 表示键值存储命令
type Command struct {
	Type      string    `json:"type"`
	Key       string    `json:"key"`
	Value     string    `json:"value,omitempty"`
	Timestamp int64     `json:"timestamp,omitempty"`
	TTL       int64     `json:"ttl,omitempty"`       // 生存时间（秒）
	BatchOps  []Command `json:"batch_ops,omitempty"` // 批量操作
}

// CommandType 命令类型
const (
	CommandSet      = "set"
	CommandGet      = "get"
	CommandDel      = "delete"
	CommandIncr     = "incr"     // 递增
	CommandExists   = "exists"   // 检查键是否存在
	CommandKeys     = "keys"     // 获取匹配模式的键
	CommandScan     = "scan"     // 扫描键值对
	CommandBatch    = "batch"    // 批量操作
	CommandExpire   = "expire"   // 设置过期时间
	CommandTTL      = "ttl"      // 获取剩余生存时间
	CommandFlushAll = "flushall" // 清空所有数据
	CommandStats    = "stats"    // 获取统计信息
	CommandSize     = "size"     // 获取数据大小
)

// KVStats 统计信息
type KVStats struct {
	mu                sync.RWMutex
	TotalOperations   int64            `json:"total_operations"`
	GetOperations     int64            `json:"get_operations"`
	SetOperations     int64            `json:"set_operations"`
	DeleteOperations  int64            `json:"delete_operations"`
	FailedOperations  int64            `json:"failed_operations"`
	LastOperationTime time.Time        `json:"last_operation_time"`
	OperationLatency  map[string]int64 `json:"operation_latency_ms"`
	KeySpaceHits      int64            `json:"keyspace_hits"`
	KeySpaceMisses    int64            `json:"keyspace_misses"`
}

// KVMetadata 元数据
type KVMetadata struct {
	mu           sync.RWMutex
	CreatedTime  time.Time        `json:"created_time"`
	LastSnapshot time.Time        `json:"last_snapshot"`
	Version      string           `json:"version"`
	Expiry       map[string]int64 `json:"expiry"` // 键的过期时间戳
}

// Apply 实现 StateMachine 接口
func (kv *KVStateMachine) Apply(data []byte) ([]byte, error) {
	start := time.Now()

	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		kv.updateStats("apply", false, time.Since(start))
		return nil, fmt.Errorf("failed to unmarshal command: %v", err)
	}

	// 设置时间戳
	if cmd.Timestamp == 0 {
		cmd.Timestamp = time.Now().Unix()
	}

	var result []byte
	var err error

	switch cmd.Type {
	case CommandSet:
		result, err = kv.handleSet(cmd)
	case CommandGet:
		result, err = kv.handleGet(cmd)
	case CommandDel:
		result, err = kv.handleDelete(cmd)
	case CommandIncr:
		result, err = kv.handleIncrement(cmd)
	case CommandExists:
		result, err = kv.handleExists(cmd)
	case CommandKeys:
		result, err = kv.handleKeys(cmd)
	case CommandScan:
		result, err = kv.handleScan(cmd)
	case CommandBatch:
		result, err = kv.handleBatch(cmd)
	case CommandExpire:
		result, err = kv.handleExpire(cmd)
	case CommandTTL:
		result, err = kv.handleTTL(cmd)
	case CommandFlushAll:
		result, err = kv.handleFlushAll(cmd)
	case CommandStats:
		result, err = kv.handleStats(cmd)
	case CommandSize:
		result, err = kv.handleSize(cmd)
	default:
		err = fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	// 更新统计信息
	kv.updateStats(cmd.Type, err == nil, time.Since(start))

	return result, err
}

// handleSet 处理设置命令
func (kv *KVStateMachine) handleSet(cmd Command) ([]byte, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data[cmd.Key] = cmd.Value

	// 处理 TTL
	if cmd.TTL > 0 {
		expiryTime := time.Now().Unix() + cmd.TTL
		kv.metadata.mu.Lock()
		kv.metadata.Expiry[cmd.Key] = expiryTime
		kv.metadata.mu.Unlock()
	}

	return []byte("OK"), nil
}

// handleGet 处理获取命令
func (kv *KVStateMachine) handleGet(cmd Command) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	// 检查键是否过期
	if kv.isExpired(cmd.Key) {
		kv.mu.RUnlock()
		kv.mu.Lock()
		delete(kv.data, cmd.Key)
		kv.metadata.mu.Lock()
		delete(kv.metadata.Expiry, cmd.Key)
		kv.metadata.mu.Unlock()
		kv.mu.Unlock()
		kv.mu.RLock()

		kv.stats.mu.Lock()
		kv.stats.KeySpaceMisses++
		kv.stats.mu.Unlock()
		return nil, fmt.Errorf("key not found: %s", cmd.Key)
	}

	value, exists := kv.data[cmd.Key]
	if !exists {
		kv.stats.mu.Lock()
		kv.stats.KeySpaceMisses++
		kv.stats.mu.Unlock()
		return nil, fmt.Errorf("key not found: %s", cmd.Key)
	}

	kv.stats.mu.Lock()
	kv.stats.KeySpaceHits++
	kv.stats.mu.Unlock()

	return []byte(value), nil
}

// handleDelete 处理删除命令
func (kv *KVStateMachine) handleDelete(cmd Command) ([]byte, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if _, exists := kv.data[cmd.Key]; !exists {
		return []byte("0"), nil // 键不存在
	}

	delete(kv.data, cmd.Key)
	kv.metadata.mu.Lock()
	delete(kv.metadata.Expiry, cmd.Key)
	kv.metadata.mu.Unlock()

	return []byte("1"), nil // 成功删除
}

// handleIncrement 处理递增命令
func (kv *KVStateMachine) handleIncrement(cmd Command) ([]byte, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	value, exists := kv.data[cmd.Key]
	var intVal int64 = 0

	if exists {
		if parsed, err := parseIntValue(value); err != nil {
			return nil, fmt.Errorf("value is not an integer: %s", value)
		} else {
			intVal = parsed
		}
	}

	increment := int64(1)
	if cmd.Value != "" {
		if parsed, err := parseIntValue(cmd.Value); err != nil {
			return nil, fmt.Errorf("increment value is not an integer: %s", cmd.Value)
		} else {
			increment = parsed
		}
	}

	intVal += increment
	kv.data[cmd.Key] = fmt.Sprintf("%d", intVal)

	return []byte(fmt.Sprintf("%d", intVal)), nil
}

// handleExists 处理存在检查命令
func (kv *KVStateMachine) handleExists(cmd Command) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	if kv.isExpired(cmd.Key) {
		return []byte("0"), nil
	}

	if _, exists := kv.data[cmd.Key]; exists {
		return []byte("1"), nil
	}

	return []byte("0"), nil
}

// handleKeys 处理键列表命令
func (kv *KVStateMachine) handleKeys(cmd Command) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	var keys []string
	pattern := cmd.Value // 使用 Value 字段作为模式
	if pattern == "" {
		pattern = "*" // 默认匹配所有
	}

	for key := range kv.data {
		if kv.isExpired(key) {
			continue
		}
		if matchPattern(key, pattern) {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys) // 保证顺序一致性
	return json.Marshal(keys)
}

// handleScan 处理扫描命令
func (kv *KVStateMachine) handleScan(cmd Command) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	result := make(map[string]string)
	pattern := cmd.Value
	if pattern == "" {
		pattern = "*"
	}

	for key, value := range kv.data {
		if kv.isExpired(key) {
			continue
		}
		if matchPattern(key, pattern) {
			result[key] = value
		}
	}

	return json.Marshal(result)
}

// handleBatch 处理批量操作命令
func (kv *KVStateMachine) handleBatch(cmd Command) ([]byte, error) {
	var results []string
	var errors []string

	for _, op := range cmd.BatchOps {
		var result []byte
		var err error

		switch op.Type {
		case CommandSet:
			result, err = kv.handleSet(op)
		case CommandGet:
			result, err = kv.handleGet(op)
		case CommandDel:
			result, err = kv.handleDelete(op)
		case CommandIncr:
			result, err = kv.handleIncrement(op)
		default:
			err = fmt.Errorf("unsupported batch operation: %s", op.Type)
		}

		if err != nil {
			errors = append(errors, fmt.Sprintf("%s:%s", op.Key, err.Error()))
		} else {
			results = append(results, string(result))
		}
	}

	response := map[string]interface{}{
		"results": results,
		"errors":  errors,
		"success": len(errors) == 0,
	}

	return json.Marshal(response)
}

// handleExpire 处理设置过期时间命令
func (kv *KVStateMachine) handleExpire(cmd Command) ([]byte, error) {
	kv.mu.RLock()
	_, exists := kv.data[cmd.Key]
	kv.mu.RUnlock()

	if !exists {
		return []byte("0"), nil
	}

	if ttl, err := parseIntValue(cmd.Value); err != nil {
		return nil, fmt.Errorf("invalid TTL value: %s", cmd.Value)
	} else {
		expiryTime := time.Now().Unix() + ttl
		kv.metadata.mu.Lock()
		kv.metadata.Expiry[cmd.Key] = expiryTime
		kv.metadata.mu.Unlock()
		return []byte("1"), nil
	}
}

// handleTTL 处理获取剩余生存时间命令
func (kv *KVStateMachine) handleTTL(cmd Command) ([]byte, error) {
	kv.metadata.mu.RLock()
	expiryTime, hasExpiry := kv.metadata.Expiry[cmd.Key]
	kv.metadata.mu.RUnlock()

	if !hasExpiry {
		return []byte("-1"), nil // 没有设置过期时间
	}

	currentTime := time.Now().Unix()
	if expiryTime <= currentTime {
		return []byte("-2"), nil // 已过期
	}

	ttl := expiryTime - currentTime
	return []byte(fmt.Sprintf("%d", ttl)), nil
}

// handleFlushAll 处理清空所有数据命令
func (kv *KVStateMachine) handleFlushAll(cmd Command) ([]byte, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data = make(map[string]string)

	kv.metadata.mu.Lock()
	kv.metadata.Expiry = make(map[string]int64)
	kv.metadata.mu.Unlock()

	return []byte("OK"), nil
}

// handleStats 处理获取统计信息命令
func (kv *KVStateMachine) handleStats(cmd Command) ([]byte, error) {
	kv.stats.mu.RLock()
	defer kv.stats.mu.RUnlock()

	stats := map[string]interface{}{
		"total_operations":  kv.stats.TotalOperations,
		"get_operations":    kv.stats.GetOperations,
		"set_operations":    kv.stats.SetOperations,
		"delete_operations": kv.stats.DeleteOperations,
		"failed_operations": kv.stats.FailedOperations,
		"keyspace_hits":     kv.stats.KeySpaceHits,
		"keyspace_misses":   kv.stats.KeySpaceMisses,
		"hit_rate":          kv.calculateHitRate(),
		"last_operation":    kv.stats.LastOperationTime.Format(time.RFC3339),
		"uptime_seconds":    time.Since(kv.metadata.CreatedTime).Seconds(),
		"keys_count":        len(kv.data),
	}

	return json.Marshal(stats)
}

// handleSize 处理获取数据大小命令
func (kv *KVStateMachine) handleSize(cmd Command) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	var totalSize int64
	for key, value := range kv.data {
		totalSize += int64(len(key) + len(value))
	}

	result := map[string]interface{}{
		"keys_count":     len(kv.data),
		"memory_usage":   totalSize,
		"avg_key_size":   kv.calculateAvgKeySize(),
		"avg_value_size": kv.calculateAvgValueSize(),
	}

	return json.Marshal(result)
}

// Snapshot 实现 StateMachine 接口
func (kv *KVStateMachine) Snapshot() ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	kv.metadata.mu.Lock()
	kv.metadata.LastSnapshot = time.Now()
	kv.metadata.mu.Unlock()

	// 清理过期键
	validData := make(map[string]string)
	for key, value := range kv.data {
		if !kv.isExpired(key) {
			validData[key] = value
		}
	}

	snapshot := map[string]interface{}{
		"data":      validData,
		"metadata":  kv.metadata,
		"stats":     kv.stats,
		"version":   "1.0.0",
		"timestamp": time.Now().Unix(),
	}

	return json.Marshal(snapshot)
}

// Restore 实现 StateMachine 接口
func (kv *KVStateMachine) Restore(data []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	var snapshot map[string]interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		// 兼容旧版本仅包含数据映射的快照
		var simple map[string]string
		if err2 := json.Unmarshal(data, &simple); err2 != nil {
			return fmt.Errorf("failed to unmarshal snapshot: %v", err)
		}
		kv.data = simple
		if kv.metadata == nil {
			kv.metadata = newKVMetadata()
		} else {
			*kv.metadata = *newKVMetadata()
		}
		if kv.stats == nil {
			kv.stats = newKVStats()
		} else {
			*kv.stats = *newKVStats()
		}
		return nil
	}

	// 恢复数据
	if dataMap, ok := snapshot["data"].(map[string]interface{}); ok {
		kv.data = make(map[string]string)
		for key, value := range dataMap {
			if strValue, ok := value.(string); ok {
				kv.data[key] = strValue
			}
		}
	}

	// 恢复元数据
	if metadataBytes, err := json.Marshal(snapshot["metadata"]); err == nil {
		json.Unmarshal(metadataBytes, kv.metadata)
	}

	// 恢复统计信息
	if statsBytes, err := json.Marshal(snapshot["stats"]); err == nil {
		json.Unmarshal(statsBytes, kv.stats)
	}

	return nil
}

// 辅助方法

// updateStats 更新统计信息
func (kv *KVStateMachine) updateStats(operation string, success bool, latency time.Duration) {
	kv.stats.mu.Lock()
	defer kv.stats.mu.Unlock()

	kv.stats.TotalOperations++
	kv.stats.LastOperationTime = time.Now()

	if kv.stats.OperationLatency == nil {
		kv.stats.OperationLatency = make(map[string]int64)
	}
	kv.stats.OperationLatency[operation] = latency.Milliseconds()

	if !success {
		kv.stats.FailedOperations++
		return
	}

	switch operation {
	case CommandGet:
		kv.stats.GetOperations++
	case CommandSet:
		kv.stats.SetOperations++
	case CommandDel:
		kv.stats.DeleteOperations++
	}
}

// isExpired 检查键是否过期
func (kv *KVStateMachine) isExpired(key string) bool {
	kv.metadata.mu.RLock()
	expiryTime, hasExpiry := kv.metadata.Expiry[key]
	kv.metadata.mu.RUnlock()

	if !hasExpiry {
		return false
	}

	return time.Now().Unix() > expiryTime
}

// calculateHitRate 计算命中率
func (kv *KVStateMachine) calculateHitRate() float64 {
	total := kv.stats.KeySpaceHits + kv.stats.KeySpaceMisses
	if total == 0 {
		return 0
	}
	return float64(kv.stats.KeySpaceHits) / float64(total) * 100
}

// calculateAvgKeySize 计算平均键大小
func (kv *KVStateMachine) calculateAvgKeySize() float64 {
	if len(kv.data) == 0 {
		return 0
	}

	var totalSize int
	for key := range kv.data {
		totalSize += len(key)
	}

	return float64(totalSize) / float64(len(kv.data))
}

// calculateAvgValueSize 计算平均值大小
func (kv *KVStateMachine) calculateAvgValueSize() float64 {
	if len(kv.data) == 0 {
		return 0
	}

	var totalSize int
	for _, value := range kv.data {
		totalSize += len(value)
	}

	return float64(totalSize) / float64(len(kv.data))
}

// matchPattern 简单的模式匹配（支持 * 通配符）
func matchPattern(str, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return str == pattern
	}

	// 简单的前缀/后缀匹配
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(str, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(str, prefix)
	}

	return false
}

// parseIntValue 解析整数值
func parseIntValue(value string) (int64, error) {
	var result int64
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return 0, err
	}
	return result, nil
}

// 初始化函数

// newKVStats 创建新的统计信息
func newKVStats() *KVStats {
	return &KVStats{
		OperationLatency: make(map[string]int64),
	}
}

// newKVMetadata 创建新的元数据
func newKVMetadata() *KVMetadata {
	return &KVMetadata{
		CreatedTime: time.Now(),
		Version:     "1.0.0",
		Expiry:      make(map[string]int64),
	}
}

// 原有的简单方法保持向后兼容

// Get 获取值
func (kv *KVStateMachine) Get(key string) (string, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	if kv.isExpired(key) {
		return "", false
	}

	value, exists := kv.data[key]
	return value, exists
}

// Set 设置值
func (kv *KVStateMachine) Set(key, value string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data[key] = value
}

// Delete 删除键
func (kv *KVStateMachine) Delete(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	delete(kv.data, key)
	kv.metadata.mu.Lock()
	delete(kv.metadata.Expiry, key)
	kv.metadata.mu.Unlock()
}

// Size 返回键值对数量
func (kv *KVStateMachine) Size() int {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	return len(kv.data)
}

// Keys 返回所有键
func (kv *KVStateMachine) Keys() []string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	keys := make([]string, 0, len(kv.data))
	for key := range kv.data {
		if !kv.isExpired(key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// 命令创建函数

// CreateSetCommand 创建设置命令
func CreateSetCommand(key, value string) ([]byte, error) {
	cmd := Command{
		Type:  CommandSet,
		Key:   key,
		Value: value,
	}
	return json.Marshal(cmd)
}

// CreateGetCommand 创建获取命令
func CreateGetCommand(key string) ([]byte, error) {
	cmd := Command{
		Type: CommandGet,
		Key:  key,
	}
	return json.Marshal(cmd)
}

// CreateDeleteCommand 创建删除命令
func CreateDeleteCommand(key string) ([]byte, error) {
	cmd := Command{
		Type: CommandDel,
		Key:  key,
	}
	return json.Marshal(cmd)
}

// CreateIncrementCommand 创建递增命令
func CreateIncrementCommand(key string, increment int64) ([]byte, error) {
	cmd := Command{
		Type:  CommandIncr,
		Key:   key,
		Value: fmt.Sprintf("%d", increment),
	}
	return json.Marshal(cmd)
}

// CreateExistsCommand 创建存在检查命令
func CreateExistsCommand(key string) ([]byte, error) {
	cmd := Command{
		Type: CommandExists,
		Key:  key,
	}
	return json.Marshal(cmd)
}

// CreateKeysCommand 创建键列表命令
func CreateKeysCommand(pattern string) ([]byte, error) {
	cmd := Command{
		Type:  CommandKeys,
		Value: pattern,
	}
	return json.Marshal(cmd)
}

// CreateScanCommand 创建扫描命令
func CreateScanCommand(pattern string) ([]byte, error) {
	cmd := Command{
		Type:  CommandScan,
		Value: pattern,
	}
	return json.Marshal(cmd)
}

// CreateBatchCommand 创建批量操作命令
func CreateBatchCommand(operations []Command) ([]byte, error) {
	cmd := Command{
		Type:     CommandBatch,
		BatchOps: operations,
	}
	return json.Marshal(cmd)
}

// CreateExpireCommand 创建设置过期时间命令
func CreateExpireCommand(key string, ttl int64) ([]byte, error) {
	cmd := Command{
		Type:  CommandExpire,
		Key:   key,
		Value: fmt.Sprintf("%d", ttl),
	}
	return json.Marshal(cmd)
}

// CreateTTLCommand 创建获取剩余生存时间命令
func CreateTTLCommand(key string) ([]byte, error) {
	cmd := Command{
		Type: CommandTTL,
		Key:  key,
	}
	return json.Marshal(cmd)
}

// CreateStatsCommand 创建获取统计信息命令
func CreateStatsCommand() ([]byte, error) {
	cmd := Command{
		Type: CommandStats,
	}
	return json.Marshal(cmd)
}

// CreateSizeCommand 创建获取数据大小命令
func CreateSizeCommand() ([]byte, error) {
	cmd := Command{
		Type: CommandSize,
	}
	return json.Marshal(cmd)
}

// CreateFlushAllCommand 创建清空所有数据命令
func CreateFlushAllCommand() ([]byte, error) {
	cmd := Command{
		Type: CommandFlushAll,
	}
	return json.Marshal(cmd)
}
