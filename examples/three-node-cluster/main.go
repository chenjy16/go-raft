package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chenjianyu/go-raft/examples/kvstore"
	"github.com/chenjianyu/go-raft/raft"
	"github.com/chenjianyu/go-raft/transport"
)

// ClusterNode 代表集群中的一个节点
type ClusterNode struct {
	ID                uint64
	Port              int
	Node              *raft.Node
	Storage           *raft.MemoryStorage
	StateMachine      *kvstore.KVStateMachine
	Transport         *transport.HTTPTransport
	TransportManager  *transport.TransportManager
	MembershipManager *raft.MembershipManager
	HTTPServer        *http.Server
	running           bool
	
	// 维护状态信息
	currentState raft.NodeState
	currentTerm  uint64
}

// NewClusterNode 创建一个新的集群节点
func NewClusterNode(id uint64, port int, peers map[uint64]string) *ClusterNode {
	storage := raft.NewMemoryStorage()
	sm := kvstore.NewKVStateMachine()

	config := &raft.Config{
		ID:                id,
		ElectionTimeout:   time.Second,      // 1秒选举超时
		HeartbeatInterval: 200 * time.Millisecond, // 200ms心跳间隔
		ElectionTick:      10, // 减少选举超时时间，加快选举 (1秒)
		HeartbeatTick:     2,  // 减少心跳间隔，保持更好的连接 (200ms)
		Storage:           storage,
		Applied:           0,
		MaxSizePerMsg:     1024 * 1024,
		MaxInflightMsgs:   256,
		PreVote:           true, // 启用PreVote以防止网络分区时的不必要选举
		CheckQuorum:       true, // 启用法定人数检查以提高稳定性
	}

	node := raft.NewNode(config, sm)
	
	// 设置 Raft 节点的 peers（只包含其他节点，不包含自己）
	if peers != nil && len(peers) > 0 {
		peerIDs := make([]uint64, 0, len(peers))
		for peerID := range peers {
			if peerID != id { // 不包含自己
				peerIDs = append(peerIDs, peerID)
			}
		}
		// 添加自己到 peers 中
		peerIDs = append(peerIDs, id)
		node.SetPeers(peerIDs)
		log.Printf("Node %d: Set peers to %v", id, peerIDs)
	}
	
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	
	// HTTPTransport 的 peers 也需要包含所有节点（包括自己用于状态查询）
	allPeers := make(map[uint64]string)
	for peerID, peerAddr := range peers {
		allPeers[peerID] = peerAddr
	}
	allPeers[id] = addr // 添加自己
	
	httpTransport := transport.NewHTTPTransport(id, addr, allPeers)
	tm := transport.NewTransportManager(httpTransport, node)
	mm := raft.NewMembershipManager(node)
	// 预填充成员信息（包括自身和已知的所有对等节点），确保集群大小一致
	for pid, paddr := range allPeers {
		mm.AddMemberDirect(pid, paddr)
	}
	
	cn := &ClusterNode{
		ID:                id,
		Port:              port,
		Node:              node,
		Storage:           storage,
		StateMachine:      sm,
		Transport:         httpTransport,
		TransportManager:  tm,
		MembershipManager: mm,
		currentState:      raft.StateFollower,
		currentTerm:       0,
	}

	cn.setupHTTPServer()
	return cn
}

// setupHTTPServer 设置HTTP服务器
func (cn *ClusterNode) setupHTTPServer() {
	mux := http.NewServeMux()

	// 键值存储API
	mux.HandleFunc("/kv/", cn.handleKV)
	// 集群状态API
	mux.HandleFunc("/cluster/status", cn.handleClusterStatus)
	// 成员管理API
	mux.HandleFunc("/cluster/members", cn.handleMembers)
	// 日志信息API
	mux.HandleFunc("/cluster/logs", cn.handleLogs)
	// 快照API
	mux.HandleFunc("/cluster/snapshot", cn.handleSnapshot)

	cn.HTTPServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cn.Port+1000), // HTTP端口 = RPC端口 + 1000
		Handler: mux,
	}
}

// handleKV 处理键值存储请求
func (cn *ClusterNode) handleKV(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[4:] // 去掉 "/kv/" 前缀

	switch r.Method {
	case "GET":
		// 对于读取操作，我们可以直接从状态机读取（如果需要强一致性读取，可以先提议一个空操作）
		// 但为了简化，这里直接读取本地状态
		value, exists := cn.StateMachine.Get(key)
		if !exists {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": key, "value": value})

	case "PUT":
		var req struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		cmd, err := kvstore.CreateSetCommand(key, req.Value)
		if err != nil {
			http.Error(w, "Failed to create command", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := cn.Node.Propose(ctx, cmd); err != nil {
			http.Error(w, fmt.Sprintf("Failed to propose: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "OK"})

	case "DELETE":
		cmd, err := kvstore.CreateDeleteCommand(key)
		if err != nil {
			http.Error(w, "Failed to create command", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := cn.Node.Propose(ctx, cmd); err != nil {
			http.Error(w, fmt.Sprintf("Failed to propose: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "OK"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClusterStatus 处理集群状态请求
func (cn *ClusterNode) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	// 从 storage 获取状态信息
	hardState, _ := cn.Storage.InitialState()
	lastIndex, _ := cn.Storage.LastIndex()
	
	status := map[string]interface{}{
		"node_id":      cn.ID,
		"state":        cn.currentState.String(),
		"leader_id":    cn.MembershipManager.GetLeader(),
		"is_leader":    cn.MembershipManager.IsLeader(),
		"term":         hardState.Term,
		"commit_index": hardState.Commit,
		"last_index":   lastIndex,
		"cluster_size": cn.MembershipManager.GetClusterSize(),
		"quorum_size":  cn.MembershipManager.GetQuorumSize(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleMembers 处理成员列表请求
func (cn *ClusterNode) handleMembers(w http.ResponseWriter, r *http.Request) {
	members := cn.MembershipManager.GetMembers()
	memberList := make([]map[string]interface{}, 0, len(members))

	for id, member := range members {
		memberList = append(memberList, map[string]interface{}{
			"id":      id,
			"address": member.Address,
			"status":  member.Status.String(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(memberList)
}

// handleLogs 处理日志信息请求
func (cn *ClusterNode) handleLogs(w http.ResponseWriter, r *http.Request) {
	firstIndex, _ := cn.Storage.FirstIndex()
	lastIndex, _ := cn.Storage.LastIndex()
	hardState, _ := cn.Storage.InitialState()
	
	info := map[string]interface{}{
		"first_index":  firstIndex,
		"last_index":   lastIndex,
		"commit_index": hardState.Commit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleSnapshot 处理快照请求
func (cn *ClusterNode) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 创建快照
	data, err := cn.StateMachine.Snapshot()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	hardState, _ := cn.Storage.InitialState()
	lastIndex, _ := cn.Storage.LastIndex()

	snapshot, err := cn.Storage.CreateSnapshot(lastIndex, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "OK",
		"index":  snapshot.Index,
		"term":   hardState.Term,
	})
}

// updateNodeState 更新节点状态（从 Ready 中获取）
func (cn *ClusterNode) updateNodeState(ready raft.Ready) {
	// 更新状态
	if ready.SoftState != nil {
		cn.currentState = ready.SoftState.State
	}
	
	// 更新任期
	if !raft.IsEmptyHardState(ready.HardState) {
		cn.currentTerm = ready.HardState.Term
	}
}

// Start 启动节点
func (cn *ClusterNode) Start() error {
	// 启动传输层
	if err := cn.TransportManager.Start(); err != nil {
		return fmt.Errorf("failed to start transport: %v", err)
	}

	// 启动Raft节点
	if err := cn.Node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %v", err)
	}

	// 启动HTTP服务器
	go func() {
		log.Printf("Node %d HTTP server listening on :%d", cn.ID, cn.Port+1000)
		if err := cn.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// 启动时钟滴答机制
	go cn.startTicker()

	// 启动Ready处理循环
	go cn.handleReady()

	cn.running = true
	log.Printf("Node %d started on port %d (HTTP: %d)", cn.ID, cn.Port, cn.Port+1000)
	return nil
}

// startTicker 启动时钟滴答机制
func (cn *ClusterNode) startTicker() {
	ticker := time.NewTicker(100 * time.Millisecond) // 100ms 滴答间隔
	defer ticker.Stop()

	for cn.running {
		select {
		case <-ticker.C:
			cn.Node.Tick()
		case <-time.After(time.Second):
			if !cn.running {
				return
			}
		}
	}
}

// Stop 停止节点
func (cn *ClusterNode) Stop() {
	if !cn.running {
		return
	}

	cn.running = false

	// 停止HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cn.HTTPServer.Shutdown(ctx)

	// 停止传输层
	cn.TransportManager.Stop()

	// 停止Raft节点
	cn.Node.Stop()

	log.Printf("Node %d stopped", cn.ID)
}

// handleReady 处理Ready事件
func (cn *ClusterNode) handleReady() {
	for cn.running {
		select {
		case ready := <-cn.Node.Ready():
			log.Printf("Node %d: Received Ready event - Messages: %d, Entries: %d, CommittedEntries: %d", 
				cn.ID, len(ready.Messages), len(ready.Entries), len(ready.CommittedEntries))
			
			// 更新本地状态（包括 leader，可从 SoftState 读取），仅在变化时记录
			if ready.SoftState != nil {
				if cn.currentState != ready.SoftState.State {
					log.Printf("Node %d: State changed to %s", cn.ID, ready.SoftState.State)
				}
				cn.currentState = ready.SoftState.State
			}

			// 先持久化硬状态和日志条目，确保消息发送前持久化
			if !raft.IsEmptyHardState(ready.HardState) {
				if cn.currentTerm != ready.HardState.Term {
					log.Printf("Node %d: Term changed to %d", cn.ID, ready.HardState.Term)
					cn.currentTerm = ready.HardState.Term
				}
				log.Printf("Node %d: Persisting hard state - Term: %d, Commit: %d", 
					cn.ID, ready.HardState.Term, ready.HardState.Commit)
				cn.Storage.SetHardState(ready.HardState)
			}
			if len(ready.Entries) > 0 {
				log.Printf("Node %d: Appending %d entries", cn.ID, len(ready.Entries))
				cn.Storage.Append(ready.Entries)
			}

			// 应用快照
			if ready.Snapshot != nil && !raft.IsEmptySnapshot(ready.Snapshot) {
				log.Printf("Node %d: Applying snapshot", cn.ID)
				cn.Storage.ApplySnapshot(ready.Snapshot)
				cn.StateMachine.Restore(ready.Snapshot.Data)
			}

			// 发送消息（在持久化之后）
			if len(ready.Messages) > 0 {
				log.Printf("Node %d: Sending %d messages", cn.ID, len(ready.Messages))
				cn.TransportManager.SendMessages(ready.Messages)
			}

			// 应用已提交的日志条目
			if len(ready.CommittedEntries) > 0 {
				log.Printf("Node %d: Applying %d committed entries", cn.ID, len(ready.CommittedEntries))
			}
			for _, entry := range ready.CommittedEntries {
				log.Printf("Node %d: Processing entry %d, Type: %d, Data length: %d", 
					cn.ID, entry.Index, entry.Type, len(entry.Data))
				if entry.Type == raft.EntryNormal && len(entry.Data) > 0 {
					result, err := cn.StateMachine.Apply(entry.Data)
					if err != nil {
						log.Printf("Node %d: Failed to apply entry %d: %v", cn.ID, entry.Index, err)
					} else {
						log.Printf("Node %d: Applied entry %d: %s", cn.ID, entry.Index, string(result))
					}
				}
			}

			// 通知Raft已处理完Ready
			cn.Node.Advance()
		}
	}
}

// Cluster 代表整个集群
type Cluster struct {
	nodes []*ClusterNode
	mu    sync.RWMutex
}

// NewCluster 创建一个新的集群
func NewCluster() *Cluster {
	return &Cluster{
		nodes: make([]*ClusterNode, 0),
	}
}

// AddNode 添加节点到集群
func (c *Cluster) AddNode(id uint64, port int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 构建当前的 peers 映射（包括即将添加的节点）
	allPeers := make(map[uint64]string)
	for _, existingNode := range c.nodes {
		allPeers[existingNode.ID] = fmt.Sprintf("127.0.0.1:%d", existingNode.Port)
	}
	allPeers[id] = fmt.Sprintf("127.0.0.1:%d", port)

	// 创建新节点，传入完整的 peers 信息
	node := NewClusterNode(id, port, allPeers)
	c.nodes = append(c.nodes, node)
	
	// 更新所有现有节点的 peers 信息
	for _, existingNode := range c.nodes[:len(c.nodes)-1] {
		existingNode.Transport.UpdatePeers(allPeers)
		// 将新节点添加到现有节点的成员管理器中
		existingNode.MembershipManager.AddMemberDirect(id, fmt.Sprintf("127.0.0.1:%d", port))
		
		// 更新现有节点的Raft peers配置
		peerIDs := make([]uint64, 0, len(allPeers))
		for peerID := range allPeers {
			peerIDs = append(peerIDs, peerID)
		}
		existingNode.Node.SetPeers(peerIDs)
		log.Printf("Updated Node %d peers to %v", existingNode.ID, peerIDs)
	}
}

// initializeClusterMembers 初始化集群成员信息
func (c *Cluster) initializeClusterMembers() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 等待一段时间让所有节点稳定
	time.Sleep(2 * time.Second)
	
	// 不再强制触发本地选举，交由超时和心跳自然形成Leader
	log.Printf("Cluster initialization complete; waiting for natural leader election")
}

// Start 启动集群
func (c *Cluster) Start() error {
	// 首先初始化集群成员信息
		c.mu.RLock()
	nodes := make([]*ClusterNode, len(c.nodes))
	copy(nodes, c.nodes)
	c.mu.RUnlock()

	// 启动所有节点
	for _, node := range nodes {
		if err := node.Start(); err != nil {
			return fmt.Errorf("failed to start node %d: %v", node.ID, err)
		}
	}

	time.Sleep(3 * time.Second) // 等待HTTP服务器启动

	// 初始化集群成员信息
	c.initializeClusterMembers()

	return nil
}

// Stop 停止集群
func (c *Cluster) Stop() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, node := range c.nodes {
		node.Stop()
	}
}

// GetNode 获取指定ID的节点
func (c *Cluster) GetNode(id uint64) *ClusterNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, node := range c.nodes {
		if node.ID == id {
			return node
		}
	}
	return nil
}

// GetLeader 获取Leader节点
func (c *Cluster) GetLeader() *ClusterNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, node := range c.nodes {
		if node.MembershipManager.IsLeader() {
			return node
		}
	}
	return nil
}

// GetNodes 获取所有节点
func (c *Cluster) GetNodes() []*ClusterNode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodes := make([]*ClusterNode, len(c.nodes))
	copy(nodes, c.nodes)
	return nodes
}

func main() {
	// 创建三节点集群
	cluster := NewCluster()
	cluster.AddNode(1, 18001)
	cluster.AddNode(2, 18002)
	cluster.AddNode(3, 18003)

	// 启动集群
	if err := cluster.Start(); err != nil {
		log.Fatalf("Failed to start cluster: %v", err)
	}

	fmt.Println("=== 三节点Raft集群已启动 ===")
	fmt.Println("节点信息:")
	fmt.Println("  Node 1: RPC端口 18001, HTTP端口 19001")
	fmt.Println("  Node 2: RPC端口 18002, HTTP端口 19002")
	fmt.Println("  Node 3: RPC端口 18003, HTTP端口 19003")
	fmt.Println()
	fmt.Println("HTTP API端点:")
	fmt.Println("  GET    /kv/{key}           - 获取键值")
	fmt.Println("  PUT    /kv/{key}           - 设置键值")
	fmt.Println("  DELETE /kv/{key}           - 删除键值")
	fmt.Println("  GET    /cluster/status     - 获取集群状态")
	fmt.Println("  GET    /cluster/members    - 获取成员列表")
	fmt.Println("  GET    /cluster/logs       - 获取日志信息")
	fmt.Println("  POST   /cluster/snapshot   - 创建快照")
	fmt.Println()

	// 等待集群稳定
	time.Sleep(3 * time.Second)

	// 启动功能测试 - 检查环境变量是否跳过测试
	if os.Getenv("RUN_TESTS") != "0" {
		go runTests(cluster)
	} else {
		fmt.Println("测试已跳过 (RUN_TESTS=0)")
	}

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n正在关闭集群...")
	cluster.Stop()
	fmt.Println("集群已关闭")
}

// runTests 运行功能测试
func runTests(cluster *Cluster) {
    time.Sleep(5 * time.Second) // 等待集群稳定

    fmt.Println("=== 开始功能测试 ===")

    // 测试1: 集群状态
    fmt.Println("\n1. 集群状态测试")
    testClusterStatus(cluster)

    // 测试2: 键值存储
    fmt.Println("\n2. 键值存储测试")
    testKeyValueOperations(cluster)

    // 测试3: 一致性测试
    fmt.Println("\n3. 数据一致性测试")
    testDataConsistency(cluster)

    // 测试4: 快照功能综合测试 (跳过)
    fmt.Println("\n4. 快照功能综合测试 (跳过)")

    // 测试5: Leader选举
    fmt.Println("\n5. Leader选举测试")
    testLeaderElection(cluster)

    // 测试6: 原始快照功能
    fmt.Println("\n6. 原始快照功能测试")
    testSnapshot(cluster)

    // 新增: 多节点快照综合测试 (跳过)
    fmt.Println("\n7. 多节点快照综合测试 (跳过)")

    fmt.Println("\n=== 功能测试完成 ===")
}

// testClusterStatus 测试集群状态
func testClusterStatus(cluster *Cluster) {
	for _, node := range cluster.GetNodes() {
		url := fmt.Sprintf("http://localhost:%d/cluster/status", node.Port+1000)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("  节点 %d: 无法获取状态 - %v\n", node.ID, err)
			continue
		}

		var status map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()

		fmt.Printf("  节点 %d: 状态=%s, 是否Leader=%v, 任期=%v\n",
			node.ID, status["state"], status["is_leader"], status["term"])
	}
}

// testKeyValueOperations 测试键值操作
func testKeyValueOperations(cluster *Cluster) {
	leader := cluster.GetLeader()
	if leader == nil {
		fmt.Println("  错误: 没有找到Leader节点")
		return
	}

	baseURL := fmt.Sprintf("http://localhost:%d", leader.Port+1000)

	// 设置键值对
	testCases := []struct {
		key   string
		value string
	}{
		{"name", "raft-cluster"},
		{"version", "1.0.0"},
		{"node_count", "3"},
	}

	for _, tc := range testCases {
		// PUT操作
		data := fmt.Sprintf(`{"value": "%s"}`, tc.value)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/kv/%s", baseURL, tc.key), 
			strings.NewReader(data))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  设置 %s=%s 失败: %v\n", tc.key, tc.value, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == 200 {
			fmt.Printf("  设置 %s=%s 成功\n", tc.key, tc.value)
		} else {
			fmt.Printf("  设置 %s=%s 失败: HTTP %d\n", tc.key, tc.value, resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond) // 等待复制
	}

	time.Sleep(1 * time.Second) // 等待复制完成

	// 获取键值对
	for _, tc := range testCases {
		resp, err := http.Get(fmt.Sprintf("%s/kv/%s", baseURL, tc.key))
		if err != nil {
			fmt.Printf("  获取 %s 失败: %v\n", tc.key, err)
			continue
		}

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result["value"] == tc.value {
			fmt.Printf("  获取 %s=%s 成功\n", tc.key, result["value"])
		} else {
			fmt.Printf("  获取 %s 失败: 期望=%s, 实际=%s\n", tc.key, tc.value, result["value"])
		}
	}
}

// testDataConsistency 测试数据一致性
func testDataConsistency(cluster *Cluster) {
	leader := cluster.GetLeader()
	if leader == nil {
		fmt.Println("  错误: 没有找到Leader节点")
		return
	}

	// 在Leader上设置一个键值
	baseURL := fmt.Sprintf("http://localhost:%d", leader.Port+1000)
	data := `{"value": "consistency-test"}`
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/kv/test-key", baseURL), 
		strings.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  设置测试键失败: %v\n", err)
		return
	}
	resp.Body.Close()

	time.Sleep(2 * time.Second) // 等待复制

	// 检查所有节点是否都有相同的值
	consistent := true
	for _, node := range cluster.GetNodes() {
		nodeURL := fmt.Sprintf("http://localhost:%d/kv/test-key", node.Port+1000)
		resp, err := http.Get(nodeURL)
		if err != nil {
			fmt.Printf("  节点 %d: 无法获取值 - %v\n", node.ID, err)
			consistent = false
			continue
		}

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result["value"] == "consistency-test" {
			fmt.Printf("  节点 %d: 数据一致 ✓\n", node.ID)
		} else {
			fmt.Printf("  节点 %d: 数据不一致 ✗ (值: %s)\n", node.ID, result["value"])
			consistent = false
		}
	}

	if consistent {
		fmt.Println("  数据一致性测试通过 ✓")
	} else {
		fmt.Println("  数据一致性测试失败 ✗")
	}
}

// testLeaderElection 测试Leader选举
func testLeaderElection(cluster *Cluster) {
	// 找到当前Leader
	currentLeader := cluster.GetLeader()
	if currentLeader == nil {
		fmt.Println("  错误: 没有找到当前Leader")
		return
	}

	fmt.Printf("  当前Leader: 节点 %d\n", currentLeader.ID)

	// 停止当前Leader
	fmt.Printf("  停止节点 %d...\n", currentLeader.ID)
	currentLeader.Stop()

	// 等待新Leader选举
	fmt.Println("  等待新Leader选举...")
	time.Sleep(10 * time.Second)

	// 检查新Leader
	newLeader := cluster.GetLeader()
	if newLeader == nil {
		fmt.Println("  Leader选举失败: 没有新Leader ✗")
		return
	}

	if newLeader.ID != currentLeader.ID {
		fmt.Printf("  新Leader选举成功: 节点 %d ✓\n", newLeader.ID)
	} else {
		fmt.Println("  Leader选举测试失败: Leader没有改变 ✗")
	}

	// 重启之前的Leader
	fmt.Printf("  重启节点 %d...\n", currentLeader.ID)
	if err := currentLeader.Start(); err != nil {
		fmt.Printf("  重启节点 %d 失败: %v\n", currentLeader.ID, err)
	} else {
		fmt.Printf("  节点 %d 重启成功\n", currentLeader.ID)
	}
}

// testSnapshot 测试快照功能
func testSnapshot(cluster *Cluster) {
	leader := cluster.GetLeader()
	if leader == nil {
		fmt.Println("  错误: 没有找到Leader节点")
		return
	}

	baseURL := fmt.Sprintf("http://localhost:%d", leader.Port+1000)

	// 创建快照
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/cluster/snapshot", baseURL), nil)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  创建快照失败: %v\n", err)
		return
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Printf("  快照创建成功: 索引=%v, 任期=%v ✓\n", 
			result["index"], result["term"])
	} else {
		fmt.Printf("  快照创建失败: HTTP %d ✗\n", resp.StatusCode)
	}
}