package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"

	"github.com/chenjianyu/go-raft/raft"
	"github.com/chenjianyu/go-raft/raft/logging"
)

// Transport 网络传输接口
type Transport interface {
	// Send 发送消息到指定节点
	Send(nodeID uint64, msg raft.Message) error

	// SetHandler 设置消息处理器
	SetHandler(handler MessageHandler)

	// Start 启动传输层
	Start() error

	// Stop 停止传输层
	Stop() error
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(msg raft.Message) error
}

// HTTPTransport HTTP 传输实现
type HTTPTransport struct {
	mu           sync.RWMutex
	nodeID       uint64
	addr         string
	peers        map[uint64]string // nodeID -> address
	handler      MessageHandler
	server       *http.Server
	rpcServer    *rpc.Server
	clients      map[uint64]*rpc.Client
	stopped      bool
	eventEmitter logging.EventEmitter
}

// NewHTTPTransport 创建 HTTP 传输
func NewHTTPTransport(nodeID uint64, addr string, peers map[uint64]string) *HTTPTransport {
	return &HTTPTransport{
		nodeID:  nodeID,
		addr:    addr,
		peers:   peers,
		clients: make(map[uint64]*rpc.Client),
	}
}

// SetEventEmitter 设置事件发射器
func (ht *HTTPTransport) SetEventEmitter(emitter logging.EventEmitter) {
	ht.mu.Lock()
	defer ht.mu.Unlock()
	ht.eventEmitter = emitter
}

// emitEvent 发射事件的便利方法
func (ht *HTTPTransport) emitEvent(event logging.Event) {
	if ht.eventEmitter != nil {
		ht.eventEmitter.EmitEvent(event)
	}
}

// RaftRPC RPC 服务
type RaftRPC struct {
	transport *HTTPTransport
}

// SendMessage RPC 方法
func (r *RaftRPC) SendMessage(msg raft.Message, reply *bool) error {
	if r.transport.handler != nil {
		err := r.transport.handler.HandleMessage(msg)
		*reply = err == nil
		return err
	}
	*reply = false
	return fmt.Errorf("no message handler set")
}

// Send 实现 Transport 接口
func (ht *HTTPTransport) Send(nodeID uint64, msg raft.Message) error {
	ht.mu.RLock()
	if ht.stopped {
		ht.mu.RUnlock()
		return fmt.Errorf("transport stopped")
	}

	client, exists := ht.clients[nodeID]
	if !exists {
		ht.mu.RUnlock()

		// 创建新的客户端连接
		addr, ok := ht.peers[nodeID]
		if !ok {
			return fmt.Errorf("unknown node: %d", nodeID)
		}

		var err error
		client, err = rpc.DialHTTP("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to connect to node %d: %v", nodeID, err)
		}

		ht.mu.Lock()
		if !ht.stopped {
			ht.clients[nodeID] = client
		} else {
			client.Close()
			ht.mu.Unlock()
			return fmt.Errorf("transport stopped")
		}
		ht.mu.Unlock()
	} else {
		ht.mu.RUnlock()
	}

	var reply bool
	err := client.Call("RaftRPC.SendMessage", msg, &reply)
	if err != nil {
		// 连接失败，移除客户端
		ht.mu.Lock()
		delete(ht.clients, nodeID)
		ht.mu.Unlock()
		client.Close()
		return fmt.Errorf("failed to send message to node %d: %v", nodeID, err)
	}

	if !reply {
		return fmt.Errorf("message rejected by node %d", nodeID)
	}

	return nil
}

// SetHandler 实现 Transport 接口
func (ht *HTTPTransport) SetHandler(handler MessageHandler) {
	ht.mu.Lock()
	defer ht.mu.Unlock()
	ht.handler = handler
}

// Start 实现 Transport 接口
func (ht *HTTPTransport) Start() error {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	if ht.stopped {
		// 允许在 Stop 之后重新启动
		ht.stopped = false
	}

	// 创建 RPC 服务器
	ht.rpcServer = rpc.NewServer()
	raftRPC := &RaftRPC{transport: ht}
	ht.rpcServer.Register(raftRPC)

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle(rpc.DefaultRPCPath, ht.rpcServer)

	ht.server = &http.Server{
		Addr:    ht.addr,
		Handler: mux,
	}

	// 启动监听
	listener, err := net.Listen("tcp", ht.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", ht.addr, err)
	}

	go func() {
		if err := ht.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// 发射服务器错误事件
			if ht.eventEmitter != nil {
				event := &logging.ErrorEvent{
					BaseEvent: logging.BaseEvent{
						Type:      logging.EventError,
						Timestamp: time.Now(),
						NodeID:    ht.nodeID,
					},
					Error:   err,
					Context: "HTTP server error",
				}
				ht.eventEmitter.EmitEvent(event)
			}
		}
	}()

	return nil
}

// Stop 实现 Transport 接口
func (ht *HTTPTransport) Stop() error {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	if ht.stopped {
		return nil
	}

	ht.stopped = true

	// 关闭所有客户端连接
	for _, client := range ht.clients {
		client.Close()
	}
	ht.clients = make(map[uint64]*rpc.Client)

	// 关闭服务器
	if ht.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return ht.server.Shutdown(ctx)
	}

	return nil
}

// UpdatePeers 更新 peers 信息
func (ht *HTTPTransport) UpdatePeers(peers map[uint64]string) {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	// 关闭不再需要的客户端连接
	for nodeID, client := range ht.clients {
		if _, exists := peers[nodeID]; !exists {
			client.Close()
			delete(ht.clients, nodeID)
		}
	}

	// 更新 peers 映射
	ht.peers = make(map[uint64]string)
	for nodeID, addr := range peers {
		ht.peers[nodeID] = addr
	}
}

// InMemoryTransport 内存传输实现（用于测试）
type InMemoryTransport struct {
	mu      sync.RWMutex
	nodeID  uint64
	handler MessageHandler
	peers   map[uint64]*InMemoryTransport
	stopped bool
}

// NewInMemoryTransport 创建内存传输
func NewInMemoryTransport(nodeID uint64) *InMemoryTransport {
	return &InMemoryTransport{
		nodeID: nodeID,
		peers:  make(map[uint64]*InMemoryTransport),
	}
}

// Connect 连接到其他节点
func (imt *InMemoryTransport) Connect(nodeID uint64, peer *InMemoryTransport) {
	imt.mu.Lock()
	defer imt.mu.Unlock()

	imt.peers[nodeID] = peer
}

// Send 实现 Transport 接口
func (imt *InMemoryTransport) Send(nodeID uint64, msg raft.Message) error {
	imt.mu.RLock()
	if imt.stopped {
		imt.mu.RUnlock()
		return fmt.Errorf("transport stopped")
	}

	peer, exists := imt.peers[nodeID]
	imt.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unknown node: %d", nodeID)
	}

	// 异步发送消息
	go func() {
		peer.mu.RLock()
		handler := peer.handler
		stopped := peer.stopped
		peer.mu.RUnlock()

		if !stopped && handler != nil {
			handler.HandleMessage(msg)
		}
	}()

	return nil
}

// SetHandler 实现 Transport 接口
func (imt *InMemoryTransport) SetHandler(handler MessageHandler) {
	imt.mu.Lock()
	defer imt.mu.Unlock()
	imt.handler = handler
}

// Start 实现 Transport 接口
func (imt *InMemoryTransport) Start() error {
	return nil
}

// Stop 实现 Transport 接口
func (imt *InMemoryTransport) Stop() error {
	imt.mu.Lock()
	defer imt.mu.Unlock()

	imt.stopped = true
	imt.peers = make(map[uint64]*InMemoryTransport)
	return nil
}

// TransportManager 传输管理器
type TransportManager struct {
	transport Transport
	node      *raft.Node
}

// NewTransportManager 创建传输管理器
func NewTransportManager(transport Transport, node *raft.Node) *TransportManager {
	tm := &TransportManager{
		transport: transport,
		node:      node,
	}

	transport.SetHandler(tm)
	return tm
}

// HandleMessage 实现 MessageHandler 接口
func (tm *TransportManager) HandleMessage(msg raft.Message) error {
	return tm.node.Step(context.Background(), msg)
}

// SendMessages 发送消息列表
func (tm *TransportManager) SendMessages(messages []raft.Message) {
	for _, msg := range messages {
		if msg.To != 0 {
			go func(m raft.Message) {
				if err := tm.transport.Send(m.To, m); err != nil {
					// 如果传输层支持事件发射器，则发射错误事件
					if httpTransport, ok := tm.transport.(*HTTPTransport); ok && httpTransport.eventEmitter != nil {
						event := &logging.ErrorEvent{
							BaseEvent: logging.BaseEvent{
								Type:      logging.EventError,
								Timestamp: time.Now(),
								NodeID:    httpTransport.nodeID,
							},
							Error:   err,
							Context: fmt.Sprintf("Failed to send message to node %d", m.To),
						}
						httpTransport.eventEmitter.EmitEvent(event)
					}
				}
			}(msg)
		}
	}
}

// Start 启动传输管理器
func (tm *TransportManager) Start() error {
	return tm.transport.Start()
}

// Stop 停止传输管理器
func (tm *TransportManager) Stop() error {
	return tm.transport.Stop()
}
