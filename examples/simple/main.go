package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/chenjianyu/go-raft/examples/kvstore"
	"github.com/chenjianyu/go-raft/raft"
	"github.com/chenjianyu/go-raft/transport"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <node-id> <port> [peer-ports...]")
		fmt.Println("Example: go run main.go 1 8001 8002 8003")
		os.Exit(1)
	}

	// 解析命令行参数
	nodeID, err := strconv.ParseUint(os.Args[1], 10, 64)
	if err != nil {
		log.Fatalf("Invalid node ID: %v", err)
	}

	port := os.Args[2]
	addr := "localhost:" + port

	// 构建集群成员配置
	peers := make(map[uint64]string)
	for i, peerPort := range os.Args[3:] {
		peerID := uint64(i + 2) // 从 2 开始编号
		if peerID >= nodeID {
			peerID++ // 跳过自己的 ID
		}
		peers[peerID] = "localhost:" + peerPort
	}

	// 创建存储
	storage := raft.NewMemoryStorage()

	// 创建状态机
	sm := kvstore.NewKVStateMachine()

	// 创建 Raft 配置
	config := &raft.Config{
		ID:              nodeID,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		Applied:         0,
		MaxSizePerMsg:   1024 * 1024,
		MaxInflightMsgs: 256,
	}

	// 创建 Raft 节点
	node := raft.NewNode(config, sm)

	// 创建传输层
	trans := transport.NewHTTPTransport(nodeID, addr, peers)
	tm := transport.NewTransportManager(trans, node)

	// 启动传输层
	if err := tm.Start(); err != nil {
		log.Fatalf("Failed to start transport: %v", err)
	}

	// 启动 Raft 节点
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	fmt.Printf("Node %d started on %s\n", nodeID, addr)
	fmt.Printf("Peers: %v\n", peers)

	// 启动消息处理循环
	go func() {
		for {
			select {
			case ready := <-node.Ready():
				// 发送消息
				if len(ready.Messages) > 0 {
					tm.SendMessages(ready.Messages)
				}

				// 持久化硬状态和日志条目
				if !raft.IsEmptyHardState(ready.HardState) {
					storage.SetHardState(ready.HardState)
				}
				if len(ready.Entries) > 0 {
					storage.Append(ready.Entries)
				}

				// 应用快照
				if ready.Snapshot != nil && !raft.IsEmptySnapshot(ready.Snapshot) {
					storage.ApplySnapshot(ready.Snapshot)
					sm.Restore(ready.Snapshot.Data)
				}

				// 应用已提交的日志条目
				for _, entry := range ready.CommittedEntries {
					if entry.Type == raft.EntryNormal && len(entry.Data) > 0 {
						result, err := sm.Apply(entry.Data)
						if err != nil {
							log.Printf("Failed to apply entry: %v", err)
						} else {
							log.Printf("Applied entry %d: %s", entry.Index, string(result))
						}
					}
				}

				// 通知 Raft 已处理完 Ready
				node.Advance()

			case <-time.After(100 * time.Millisecond):
				// 定期 tick
				node.Tick()
			}
		}
	}()

	// 示例：定期提交一些键值对
	go func() {
		time.Sleep(5 * time.Second) // 等待集群稳定

		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key%d", i)
			value := fmt.Sprintf("value%d-%d", i, nodeID)

			cmd, err := kvstore.CreateSetCommand(key, value)
			if err != nil {
				log.Printf("Failed to create command: %v", err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = node.Propose(ctx, cmd)
			cancel()

			if err != nil {
				log.Printf("Failed to propose: %v", err)
			} else {
				log.Printf("Proposed: %s = %s", key, value)
			}

			time.Sleep(2 * time.Second)
		}
	}()

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")

	// 停止节点
	node.Stop()
	tm.Stop()

	fmt.Println("Node stopped")
}
