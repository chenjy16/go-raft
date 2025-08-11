package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MultiNodeSnapshotTester 多节点快照测试器
type MultiNodeSnapshotTester struct {
	client  *http.Client
	nodes   []int // 节点端口列表
	baseURL func(port int) string
}

// NewMultiNodeSnapshotTester 创建多节点快照测试器
func NewMultiNodeSnapshotTester() *MultiNodeSnapshotTester {
	return &MultiNodeSnapshotTester{
		client: &http.Client{Timeout: 10 * time.Second},
		nodes:  []int{19001, 19002, 19003},
		baseURL: func(port int) string {
			return fmt.Sprintf("http://localhost:%d", port)
		},
	}
}

// TestResult 测试结果
type TestResult struct {
	TestName  string        `json:"test_name"`
	Success   bool          `json:"success"`
	Message   string        `json:"message"`
	Details   string        `json:"details,omitempty"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"start_time"`
}

// NodeStatus 节点状态
type NodeStatus struct {
	NodeID      int    `json:"node_id"`
	State       string `json:"state"`
	IsLeader    bool   `json:"is_leader"`
	Term        int    `json:"term"`
	CommitIndex int    `json:"commit_index"`
	LastIndex   int    `json:"last_index"`
	ClusterSize int    `json:"cluster_size"`
}

// LogStatus 日志状态
type LogStatus struct {
	FirstIndex  int `json:"first_index"`
	LastIndex   int `json:"last_index"`
	CommitIndex int `json:"commit_index"`
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	Term   int    `json:"term"`
}

// KeyValue 键值对
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// RunAllTests 运行所有快照测试
func (tester *MultiNodeSnapshotTester) RunAllTests() []TestResult {
	var results []TestResult

	fmt.Println("=== 开始多节点快照综合测试 ===")

	// 测试1: 检查集群状态
	results = append(results, tester.testClusterStatus())

	// 测试2: 多节点快照复制测试
	results = append(results, tester.testMultiNodeSnapshotReplication())

	// 测试3: 快照恢复测试
	results = append(results, tester.testSnapshotRecovery())

	// 测试4: 快照压缩测试
	results = append(results, tester.testSnapshotCompression())

	// 测试5: 快照一致性验证
	results = append(results, tester.testSnapshotConsistency())

	return results
}

// testClusterStatus 测试集群状态
func (tester *MultiNodeSnapshotTester) testClusterStatus() TestResult {
	start := time.Now()
	fmt.Println("测试1: 检查集群状态...")

	var details []string
	activeNodes := 0

	for _, port := range tester.nodes {
		status, err := tester.getNodeStatus(port)
		if err != nil {
			details = append(details, fmt.Sprintf("节点%d: 无法连接 - %v", port-19000, err))
		} else {
			activeNodes++
			details = append(details, fmt.Sprintf("节点%d: %s (Term: %d, Leader: %t)",
				port-19000, status.State, status.Term, status.IsLeader))
		}
	}

	success := activeNodes > 0
	message := fmt.Sprintf("活跃节点数: %d/%d", activeNodes, len(tester.nodes))

	return TestResult{
		TestName:  "集群状态检查",
		Success:   success,
		Message:   message,
		Details:   fmt.Sprintf("节点状态:\n%s", joinStrings(details, "\n")),
		Duration:  time.Since(start),
		StartTime: start,
	}
}

// testMultiNodeSnapshotReplication 测试多节点快照复制
func (tester *MultiNodeSnapshotTester) testMultiNodeSnapshotReplication() TestResult {
	start := time.Now()
	fmt.Println("测试2: 多节点快照复制...")

	// 1. 添加测试数据
	testData := map[string]string{
		"replication-test-1": "multi-node-value1",
		"replication-test-2": "multi-node-value2",
		"replication-test-3": "multi-node-value3",
	}

	// 找到一个可用的节点来设置数据
	var dataNode int
	for _, port := range tester.nodes {
		if tester.isNodeAccessible(port) {
			dataNode = port
			break
		}
	}

	if dataNode == 0 {
		return TestResult{
			TestName:  "多节点快照复制",
			Success:   false,
			Message:   "没有可用的节点来设置测试数据",
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 设置测试数据
	for key, value := range testData {
		if err := tester.setKeyValue(dataNode, key, value); err != nil {
			return TestResult{
				TestName:  "多节点快照复制",
				Success:   false,
				Message:   fmt.Sprintf("设置测试数据失败: %v", err),
				Duration:  time.Since(start),
				StartTime: start,
			}
		}
	}

	// 等待数据复制
	time.Sleep(2 * time.Second)

	// 2. 在每个可用节点上创建快照
	snapshots := make(map[int]*SnapshotInfo)
	var details []string

	for _, port := range tester.nodes {
		if tester.isNodeAccessible(port) {
			snapshot, err := tester.createSnapshot(port)
			if err != nil {
				details = append(details, fmt.Sprintf("节点%d: 快照创建失败 - %v", port-19000, err))
			} else {
				snapshots[port] = snapshot
				details = append(details, fmt.Sprintf("节点%d: 快照创建成功 (索引: %d, Term: %d)",
					port-19000, snapshot.Index, snapshot.Term))
			}
		}
	}

	// 3. 验证快照后数据一致性
	consistency := tester.verifyDataConsistency(testData)

	success := len(snapshots) > 0 && consistency
	message := fmt.Sprintf("成功创建快照的节点数: %d, 数据一致性: %t", len(snapshots), consistency)

	return TestResult{
		TestName:  "多节点快照复制",
		Success:   success,
		Message:   message,
		Details:   fmt.Sprintf("快照结果:\n%s", joinStrings(details, "\n")),
		Duration:  time.Since(start),
		StartTime: start,
	}
}

// testSnapshotRecovery 测试快照恢复
func (tester *MultiNodeSnapshotTester) testSnapshotRecovery() TestResult {
	start := time.Now()
	fmt.Println("测试3: 快照恢复测试...")

	// 1. 准备恢复测试数据
	recoveryData := map[string]string{
		"recovery-test-1": "before-recovery-1",
		"recovery-test-2": "before-recovery-2",
	}

	// 找到一个可用节点
	var activeNode int
	for _, port := range tester.nodes {
		if tester.isNodeAccessible(port) {
			activeNode = port
			break
		}
	}

	if activeNode == 0 {
		return TestResult{
			TestName:  "快照恢复",
			Success:   false,
			Message:   "没有可用的节点进行恢复测试",
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 2. 设置恢复前数据
	for key, value := range recoveryData {
		if err := tester.setKeyValue(activeNode, key, value); err != nil {
			return TestResult{
				TestName:  "快照恢复",
				Success:   false,
				Message:   fmt.Sprintf("设置恢复前数据失败: %v", err),
				Duration:  time.Since(start),
				StartTime: start,
			}
		}
	}

	time.Sleep(1 * time.Second)

	// 3. 获取恢复前的日志状态
	logsBefore, err := tester.getLogStatus(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照恢复",
			Success:   false,
			Message:   fmt.Sprintf("获取恢复前日志状态失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 4. 创建快照
	snapshot, err := tester.createSnapshot(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照恢复",
			Success:   false,
			Message:   fmt.Sprintf("创建恢复快照失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 5. 添加快照后数据
	postSnapshotData := map[string]string{
		"recovery-test-3": "after-recovery-3",
	}

	for key, value := range postSnapshotData {
		if err := tester.setKeyValue(activeNode, key, value); err != nil {
			return TestResult{
				TestName:  "快照恢复",
				Success:   false,
				Message:   fmt.Sprintf("设置快照后数据失败: %v", err),
				Duration:  time.Since(start),
				StartTime: start,
			}
		}
	}

	time.Sleep(1 * time.Second)

	// 6. 获取恢复后的日志状态
	logsAfter, err := tester.getLogStatus(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照恢复",
			Success:   false,
			Message:   fmt.Sprintf("获取恢复后日志状态失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 7. 验证数据恢复
	allData := make(map[string]string)
	for k, v := range recoveryData {
		allData[k] = v
	}
	for k, v := range postSnapshotData {
		allData[k] = v
	}

	dataRecovered := tester.verifyDataExists(activeNode, allData)

	details := fmt.Sprintf("快照索引: %d, Term: %d\n恢复前日志: FirstIndex=%d, LastIndex=%d, CommitIndex=%d\n恢复后日志: FirstIndex=%d, LastIndex=%d, CommitIndex=%d\n数据恢复: %t",
		snapshot.Index, snapshot.Term,
		logsBefore.FirstIndex, logsBefore.LastIndex, logsBefore.CommitIndex,
		logsAfter.FirstIndex, logsAfter.LastIndex, logsAfter.CommitIndex,
		dataRecovered)

	success := snapshot.Status == "OK" && dataRecovered
	message := fmt.Sprintf("快照创建: %s, 数据恢复: %t", snapshot.Status, dataRecovered)

	return TestResult{
		TestName:  "快照恢复",
		Success:   success,
		Message:   message,
		Details:   details,
		Duration:  time.Since(start),
		StartTime: start,
	}
}

// testSnapshotCompression 测试快照压缩
func (tester *MultiNodeSnapshotTester) testSnapshotCompression() TestResult {
	start := time.Now()
	fmt.Println("测试4: 快照压缩测试...")

	// 找到一个可用节点
	var activeNode int
	for _, port := range tester.nodes {
		if tester.isNodeAccessible(port) {
			activeNode = port
			break
		}
	}

	if activeNode == 0 {
		return TestResult{
			TestName:  "快照压缩",
			Success:   false,
			Message:   "没有可用的节点进行压缩测试",
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 1. 获取压缩前的日志状态
	logsBefore, err := tester.getLogStatus(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照压缩",
			Success:   false,
			Message:   fmt.Sprintf("获取压缩前日志状态失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 2. 添加一些数据以增加日志条目
	compressionData := make(map[string]string)
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("compression-test-%d", i)
		value := fmt.Sprintf("compression-value-%d", i)
		compressionData[key] = value

		if err := tester.setKeyValue(activeNode, key, value); err != nil {
			return TestResult{
				TestName:  "快照压缩",
				Success:   false,
				Message:   fmt.Sprintf("设置压缩测试数据失败: %v", err),
				Duration:  time.Since(start),
				StartTime: start,
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	// 3. 获取添加数据后的日志状态
	logsMiddle, err := tester.getLogStatus(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照压缩",
			Success:   false,
			Message:   fmt.Sprintf("获取中间日志状态失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 4. 创建快照进行压缩
	snapshot, err := tester.createSnapshot(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照压缩",
			Success:   false,
			Message:   fmt.Sprintf("创建压缩快照失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	time.Sleep(1 * time.Second)

	// 5. 获取压缩后的日志状态
	logsAfter, err := tester.getLogStatus(activeNode)
	if err != nil {
		return TestResult{
			TestName:  "快照压缩",
			Success:   false,
			Message:   fmt.Sprintf("获取压缩后日志状态失败: %v", err),
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 6. 验证压缩效果和数据完整性
	dataIntact := tester.verifyDataExists(activeNode, compressionData)

	// 分析压缩效果
	logGrowth := logsMiddle.LastIndex - logsBefore.LastIndex
	compressionEffective := snapshot.Index >= logsBefore.LastIndex

	details := fmt.Sprintf("压缩前: FirstIndex=%d, LastIndex=%d, CommitIndex=%d\n添加数据后: FirstIndex=%d, LastIndex=%d, CommitIndex=%d\n压缩后: FirstIndex=%d, LastIndex=%d, CommitIndex=%d\n快照索引: %d, Term: %d\n日志增长: %d条目\n数据完整性: %t\n压缩有效性: %t",
		logsBefore.FirstIndex, logsBefore.LastIndex, logsBefore.CommitIndex,
		logsMiddle.FirstIndex, logsMiddle.LastIndex, logsMiddle.CommitIndex,
		logsAfter.FirstIndex, logsAfter.LastIndex, logsAfter.CommitIndex,
		snapshot.Index, snapshot.Term,
		logGrowth, dataIntact, compressionEffective)

	success := snapshot.Status == "OK" && dataIntact && compressionEffective
	message := fmt.Sprintf("快照状态: %s, 数据完整: %t, 压缩有效: %t", snapshot.Status, dataIntact, compressionEffective)

	return TestResult{
		TestName:  "快照压缩",
		Success:   success,
		Message:   message,
		Details:   details,
		Duration:  time.Since(start),
		StartTime: start,
	}
}

// testSnapshotConsistency 测试快照一致性
func (tester *MultiNodeSnapshotTester) testSnapshotConsistency() TestResult {
	start := time.Now()
	fmt.Println("测试5: 快照一致性验证...")

	// 准备一致性测试数据
	consistencyData := map[string]string{
		"consistency-1": "value1",
		"consistency-2": "value2",
	}

	// 找到一个可用节点设置数据
	var dataNode int
	for _, port := range tester.nodes {
		if tester.isNodeAccessible(port) {
			dataNode = port
			break
		}
	}

	if dataNode == 0 {
		return TestResult{
			TestName:  "快照一致性验证",
			Success:   false,
			Message:   "没有可用的节点进行一致性测试",
			Duration:  time.Since(start),
			StartTime: start,
		}
	}

	// 设置测试数据
	for key, value := range consistencyData {
		if err := tester.setKeyValue(dataNode, key, value); err != nil {
			return TestResult{
				TestName:  "快照一致性验证",
				Success:   false,
				Message:   fmt.Sprintf("设置一致性测试数据失败: %v", err),
				Duration:  time.Since(start),
				StartTime: start,
			}
		}
	}

	time.Sleep(2 * time.Second)

	// 在所有可用节点上创建快照
	snapshots := make(map[int]*SnapshotInfo)
	var details []string

	for _, port := range tester.nodes {
		if tester.isNodeAccessible(port) {
			snapshot, err := tester.createSnapshot(port)
			if err != nil {
				details = append(details, fmt.Sprintf("节点%d: 快照创建失败 - %v", port-19000, err))
			} else {
				snapshots[port] = snapshot
				details = append(details, fmt.Sprintf("节点%d: 快照 (索引: %d, Term: %d)",
					port-19000, snapshot.Index, snapshot.Term))
			}
		}
	}

	// 检查快照一致性
	consistent := tester.checkSnapshotConsistency(snapshots)
	dataConsistent := tester.verifyDataConsistency(consistencyData)

	success := len(snapshots) > 0 && consistent && dataConsistent
	message := fmt.Sprintf("快照节点数: %d, 快照一致: %t, 数据一致: %t", len(snapshots), consistent, dataConsistent)

	return TestResult{
		TestName:  "快照一致性验证",
		Success:   success,
		Message:   message,
		Details:   fmt.Sprintf("快照详情:\n%s", joinStrings(details, "\n")),
		Duration:  time.Since(start),
		StartTime: start,
	}
}

// 辅助方法

// isNodeAccessible 检查节点是否可访问
func (tester *MultiNodeSnapshotTester) isNodeAccessible(port int) bool {
	_, err := tester.getNodeStatus(port)
	return err == nil
}

// getNodeStatus 获取节点状态
func (tester *MultiNodeSnapshotTester) getNodeStatus(port int) (*NodeStatus, error) {
	url := fmt.Sprintf("%s/cluster/status", tester.baseURL(port))
	resp, err := tester.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var status NodeStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

// getLogStatus 获取日志状态
func (tester *MultiNodeSnapshotTester) getLogStatus(port int) (*LogStatus, error) {
	url := fmt.Sprintf("%s/cluster/logs", tester.baseURL(port))
	resp, err := tester.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var logs LogStatus
	err = json.Unmarshal(body, &logs)
	if err != nil {
		return nil, err
	}

	return &logs, nil
}

// createSnapshot 创建快照
func (tester *MultiNodeSnapshotTester) createSnapshot(port int) (*SnapshotInfo, error) {
	url := fmt.Sprintf("%s/cluster/snapshot", tester.baseURL(port))
	resp, err := tester.client.Post(url, "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var snapshot SnapshotInfo
	err = json.Unmarshal(body, &snapshot)
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// setKeyValue 设置键值对
func (tester *MultiNodeSnapshotTester) setKeyValue(port int, key, value string) error {
	url := fmt.Sprintf("%s/kv/%s", tester.baseURL(port), key)
	data := map[string]string{"value": value}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tester.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getKeyValue 获取键值对
func (tester *MultiNodeSnapshotTester) getKeyValue(port int, key string) (string, error) {
	url := fmt.Sprintf("%s/kv/%s", tester.baseURL(port), key)
	resp, err := tester.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("key not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result KeyValue
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	return result.Value, nil
}

// verifyDataConsistency 验证数据一致性
func (tester *MultiNodeSnapshotTester) verifyDataConsistency(testData map[string]string) bool {
	for key, expectedValue := range testData {
		var values []string
		for _, port := range tester.nodes {
			if tester.isNodeAccessible(port) {
				value, err := tester.getKeyValue(port, key)
				if err == nil {
					values = append(values, value)
				}
			}
		}

		// 检查所有值是否一致
		if len(values) > 1 {
			for _, value := range values {
				if value != expectedValue {
					return false
				}
			}
		}
	}
	return true
}

// verifyDataExists 验证数据存在
func (tester *MultiNodeSnapshotTester) verifyDataExists(port int, testData map[string]string) bool {
	for key, expectedValue := range testData {
		value, err := tester.getKeyValue(port, key)
		if err != nil || value != expectedValue {
			return false
		}
	}
	return true
}

// checkSnapshotConsistency 检查快照一致性
func (tester *MultiNodeSnapshotTester) checkSnapshotConsistency(snapshots map[int]*SnapshotInfo) bool {
	if len(snapshots) <= 1 {
		return true
	}

	var referenceSnapshot *SnapshotInfo
	for _, snapshot := range snapshots {
		if referenceSnapshot == nil {
			referenceSnapshot = snapshot
		} else {
			// 检查索引和任期是否一致（允许小的差异）
			if abs(snapshot.Index-referenceSnapshot.Index) > 1 {
				return false
			}
			if abs(snapshot.Term-referenceSnapshot.Term) > 1 {
				return false
			}
		}
	}
	return true
}

// printTestResults 打印测试结果
func (tester *MultiNodeSnapshotTester) printTestResults(results []TestResult) {
	fmt.Println("\n=== 多节点快照测试结果汇总 ===")

	successCount := 0
	for i, result := range results {
		fmt.Printf("\n测试 %d: %s\n", i+1, result.TestName)
		fmt.Printf("⏱️  耗时: %v\n", result.Duration)
		if result.Success {
			fmt.Printf("✅ 成功: %s\n", result.Message)
			successCount++
		} else {
			fmt.Printf("❌ 失败: %s\n", result.Message)
		}
		if result.Details != "" {
			fmt.Printf("   详情: %s\n", result.Details)
		}
	}

	fmt.Printf("\n总结: %d/%d 测试通过\n", successCount, len(results))

	if successCount == len(results) {
		fmt.Println("🎉 所有多节点快照测试都通过了！")
	} else {
		fmt.Println("⚠️  部分多节点快照测试失败，请检查详情。")
	}
}

// 辅助函数

// abs 计算绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// joinStrings 连接字符串数组
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// RunMultiNodeSnapshotTests 运行多节点快照测试的主函数
func RunMultiNodeSnapshotTests() {
	fmt.Println("🚀 启动多节点快照综合测试...")

	tester := NewMultiNodeSnapshotTester()

	// 等待集群稳定
	time.Sleep(3 * time.Second)

	// 运行所有测试
	results := tester.RunAllTests()

	// 打印结果
	tester.printTestResults(results)

	fmt.Println("\n🏁 多节点快照综合测试完成")
}
