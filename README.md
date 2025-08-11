# Go-Raft

A production-ready Raft consensus algorithm library implemented in Go.

## 项目概述

Go-Raft 是一个用 Go 语言实现的 Raft 分布式一致性算法库。项目分为两个阶段开发：

1. **第一阶段（基础实现）**：实现了 Raft 算法的核心功能
2. **第二阶段（生产就绪）**：添加了生产环境所需的高级功能

## Features

### 第一阶段功能 ✅
- [x] **核心 Raft 算法**：领导者选举、日志复制、安全性保证
- [x] **内存存储后端**：MemoryStorage 实现
- [x] **键值存储状态机**：简单的 KV 存储示例
- [x] **HTTP RPC 传输层**：生产环境网络传输
- [x] **内存传输层**：测试环境传输实现
- [x] **基础测试用例**：核心功能测试覆盖
- [x] **完整示例程序**：从简单到复杂的使用示例

### 第二阶段功能（生产就绪）✅
- [x] **持久化存储支持**：FileStorage 文件存储实现
- [x] **快照和日志压缩**：SnapshotManager 自动快照管理
- [x] **集群成员管理**：MembershipManager 动态成员变更
- [x] **性能优化**：批处理、流水线、预投票优化
- [x] **ReadIndex/LeaseRead**：线性化读取优化
- [x] **Learner 节点支持**：非投票成员支持
- [x] **网络分区处理**：PreVote 和分区检测
- [x] **事件驱动日志系统**：结构化日志、多日志器、依赖注入
- [x] **生产级示例**：完整的生产环境示例
- [x] **监控和指标**：性能统计和监控功能

### 高级特性 ✅
- [x] **强一致性保证**：完整的 Raft 算法实现
- [x] **容错性**：支持少数节点故障
- [x] **分区容忍性**：网络分区时保持可用性
- [x] **高性能**：批处理、流水线等优化技术
- [x] **可扩展性**：支持动态添加和移除节点
- [x] **生产特性**：持久化、快照、监控等

## Quick Start

### Installation

```bash
git clone https://github.com/chenjy16/go-raft.git
cd go-raft
go mod tidy
```

### Build and Test

```bash
# Build the project
make build

# Run tests
make test

# Run tests with coverage report
make test-coverage

# Code formatting and static analysis
make check
```

### Running Examples

#### Single Node Mode
```bash
make run-single
# or
go run examples/simple/main.go 1 8001
```

#### Three Node Cluster
```bash
# Method 1: Using Makefile (recommended)
make run-example

# Method 2: Manual startup
# Terminal 1
go run examples/simple/main.go 1 8001 8002 8003

# Terminal 2  
go run examples/simple/main.go 2 8002 8001 8003

# Terminal 3
go run examples/simple/main.go 3 8003 8001 8002
```

## Architecture

### Directory Structure
```
go-raft/
├── raft/                    # 核心 Raft 实现
│   ├── types.go            # 基础类型定义
│   ├── node.go             # Raft 节点实现
│   ├── log.go              # 日志管理
│   ├── storage.go          # 存储接口和内存实现
│   ├── file_storage.go     # 文件存储实现 (第二阶段)
│   ├── snapshot.go         # 快照管理 (第二阶段)
│   ├── membership.go       # 成员管理 (第二阶段)
│   ├── performance.go      # 性能优化 (第二阶段)
│   ├── readindex.go        # ReadIndex 实现 (第二阶段)
│   ├── learner.go          # Learner 节点支持 (第二阶段)
│   ├── prevote.go          # PreVote 和网络分区处理 (第二阶段)
│   ├── errors.go           # 错误定义 (第二阶段)
│   ├── logging/            # 事件驱动日志系统 (第二阶段)
│   │   ├── README.md       # 日志系统详细文档
│   │   ├── event.go        # 事件系统定义
│   │   ├── logger.go       # 日志器实现
│   │   ├── emitter.go      # 事件发射器
│   │   └── config.go       # 配置和工厂
│   ├── di/                 # 依赖注入系统 (第二阶段)
│   │   └── module.go       # DI 容器和模块
│   ├── raft_test.go        # 基础测试
│   └── stage2_test.go      # 第二阶段测试
├── transport/               # 网络传输层
│   └── transport.go        # HTTP RPC 和内存传输实现
├── examples/                # 示例程序
│   ├── simple/             # 简单示例
│   ├── kvstore/            # 键值存储状态机
│   ├── three-node-cluster/ # 三节点集群示例
│   ├── production/         # 生产级示例 (第二阶段)
│   └── advanced_features.go # 高级功能演示 (第二阶段)
├── Makefile                # 构建脚本
├── go.mod                  # Go 模块定义
├── README.md               # 项目文档
├── LOGGING_IMPLEMENTATION.md # 日志系统实现总结
├── STAGE2_README.md        # 第二阶段功能说明
├── STAGE2_COMPLETION.md    # 第二阶段完成报告
└── PROJECT_SUMMARY.md      # 项目总结
```

### Core Components

#### 1. Raft Node (`raft.Node`)
- 实现完整的 Raft 算法逻辑
- 支持领导者选举、日志复制和安全性保证
- 提供异步消息处理机制
- 集成 ReadIndex、Learner、PreVote 等高级功能

#### 2. Storage Interface (`raft.Storage`)
- 抽象存储接口，支持多种后端
- 内存存储实现 (`MemoryStorage`)
- 文件存储实现 (`FileStorage`) - 生产环境持久化
- 支持日志条目、硬状态和快照的持久化

#### 3. Transport Layer (`transport.Transport`)
- HTTP RPC 传输（生产环境）
- 内存传输（测试环境）
- 支持可靠的消息传递

#### 4. State Machine Interface (`raft.StateMachine`)
- 用户定义的业务逻辑
- 支持快照和恢复
- 示例：键值存储状态机

#### 5. Advanced Managers (第二阶段)
- **SnapshotManager**: 自动快照管理和日志压缩
- **MembershipManager**: 集群成员动态管理
- **PerformanceOptimizer**: 性能优化（批处理、流水线）
- **ReadIndexManager**: 线性化读取优化
- **LearnerManager**: Learner 节点管理
- **PreVoteManager**: PreVote 和网络分区处理

#### 6. Logging System (事件驱动日志系统)
- **Event System**: 结构化事件定义，支持 15+ 种事件类型
- **Logger Interface**: 统一的日志器接口，支持多种实现
- **EventEmitter**: 事件发射器，提供便利的事件发射方法
- **Multiple Loggers**: ConsoleLogger、SlogLogger、MultiLogger、NopLogger
- **Configuration**: 灵活的配置系统，支持多种输出格式和目标

#### 7. Dependency Injection (依赖注入系统)
- **Container**: 轻量级 DI 容器，类型安全的服务注册和解析
- **ApplicationBuilder**: 应用构建器，支持链式配置
- **Module System**: 模块化配置，支持日志和 Raft 模块
- **Integration**: 与日志系统深度集成，简化配置

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/chenjy16/go-raft/raft"
    "github.com/chenjy16/go-raft/examples/kvstore"
    "github.com/chenjy16/go-raft/transport"
)

func main() {
    // 1. Create storage
    storage := raft.NewMemoryStorage()
    
    // 2. Create state machine
    sm := kvstore.NewKVStateMachine()
    
    // 3. Create configuration
    config := &raft.Config{
        ID:              1,
        ElectionTick:    10,
        HeartbeatTick:   1,
        Storage:         storage,
        Applied:         0,
        MaxSizePerMsg:   1024 * 1024,
        MaxInflightMsgs: 256,
    }
    
    // 4. Create node
    node := raft.NewNode(config, sm)
    
    // 5. Create transport layer
    peers := map[uint64]string{
        2: "localhost:8002",
        3: "localhost:8003",
    }
    trans := transport.NewHTTPTransport(1, "localhost:8001", peers)
    tm := transport.NewTransportManager(trans, node)
    
    // 6. Start services
    tm.Start()
    node.Start()
    defer func() {
        node.Stop()
        tm.Stop()
    }()
    
    // 7. Handle Ready state
    go func() {
        for {
            select {
            case ready := <-node.Ready():
                // Send messages
                if len(ready.Messages) > 0 {
                    tm.SendMessages(ready.Messages)
                }
                
                // Persist state
                if !raft.IsEmptyHardState(ready.HardState) {
                    storage.SetHardState(ready.HardState)
                }
                if len(ready.Entries) > 0 {
                    storage.Append(ready.Entries)
                }
                
                // Apply log entries
                for _, entry := range ready.CommittedEntries {
                    if entry.Type == raft.EntryNormal && len(entry.Data) > 0 {
                        sm.Apply(entry.Data)
                    }
                }
                
                // Notify processing complete
                node.Advance()
            }
        }
    }()
    
    // 8. Propose operations
    cmd, _ := kvstore.CreateSetCommand("key1", "value1")
    node.Propose(context.Background(), cmd)
}
```

### Key-Value Store Operations

```go
// Create commands
setCmd, _ := kvstore.CreateSetCommand("mykey", "myvalue")
getCmd, _ := kvstore.CreateGetCommand("mykey")
delCmd, _ := kvstore.CreateDeleteCommand("mykey")

// Propose operations
node.Propose(context.Background(), setCmd)
node.Propose(context.Background(), getCmd)
node.Propose(context.Background(), delCmd)
```

### Logging System Usage

#### Basic Logging Usage

```go
package main

import (
    "github.com/chenjianyu/go-raft/raft/logging"
    "github.com/chenjianyu/go-raft/raft/di"
)

func main() {
    // 1. 直接使用日志器
    logger := logging.NewConsoleLogger(logging.LevelInfo)
    emitter := logging.NewEventEmitter(logger, 1) // nodeID = 1
    
    // 发射事件
    emitter.EmitNodeStarting(map[string]interface{}{
        "config": "default",
    })
    
    // 2. 使用配置创建日志器
    config := logging.LoggerConfig{
        Type:  logging.LoggerTypeSlog,
        Level: logging.LevelInfo,
        Format: logging.FormatConfig{
            JSON: true,
            AddSource: true,
        },
    }
    
    factory := logging.NewLoggerFactory()
    logger, err := factory.CreateLogger(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. 使用依赖注入
    builder := di.NewApplicationBuilder()
    builder.WithConsoleLogging(logging.LevelInfo)
    builder.WithRaft(1)
    
    app, err := builder.Build()
    if err != nil {
        log.Fatal(err)
    }
    
    emitter, err := app.GetNodeEventEmitter()
    if err != nil {
        log.Fatal(err)
    }
    
    // 使用事件发射器
    emitter.EmitStateChanged("follower", "candidate")
    emitter.EmitElectionStarted(1)
    emitter.EmitLogAppended(10, 1, 10, time.Millisecond*5, nil)
}
```

#### Advanced Logging Configuration

```go
// 多日志器组合
consoleLogger := logging.NewConsoleLogger(logging.LevelInfo)
jsonLogger := logging.NewSlogLogger(logging.LevelDebug)
multiLogger := logging.NewMultiLogger(consoleLogger, jsonLogger)

// 使用构建器配置
builder := logging.NewLoggerBuilder()
logger := builder.
    SetType(logging.LoggerTypeSlog).
    SetLevel(logging.LevelInfo).
    SetFormat(logging.FormatConfig{JSON: true}).
    SetOutput(logging.OutputConfig{Target: "file", File: &logging.FileConfig{Path: "raft.log"}}).
    Build()

// 依赖注入配置
appBuilder := di.NewApplicationBuilder()
appBuilder.WithJSONLogging(logging.LevelDebug, "raft.log")
appBuilder.WithRaft(nodeID)

app, _ := appBuilder.Build()
```

## 第二阶段功能详解

### 1. 持久化存储支持

#### FileStorage

`FileStorage` 提供了基于文件系统的持久化存储实现：

```go
// 创建文件存储
storage, err := raft.NewFileStorage("/path/to/data")
if err != nil {
    log.Fatal(err)
}
defer storage.Close()

// 在配置中使用
config := raft.DefaultConfig()
config.Storage = storage
```

**文件结构：**
```
data/
├── hard_state.json    # 硬状态
├── entries.log        # 日志条目
└── snapshot.snap      # 快照数据
```

### 2. 快照和日志压缩

#### SnapshotManager

自动快照管理功能：

```go
// 创建快照管理器
snapshotManager := raft.NewSnapshotManager(node, storage, stateMachine)

// 配置快照阈值
snapshotManager.SetSnapshotThreshold(1000)  // 1000个条目后创建快照
snapshotManager.SetCompactThreshold(500)    // 保留最近500个条目

// 启动管理器
snapshotManager.Start()
defer snapshotManager.Stop()

// 手动创建快照
err := snapshotManager.CreateSnapshotNow()
```

### 3. 集群成员管理

#### MembershipManager

动态集群成员管理：

```go
// 创建成员管理器
membershipManager := raft.NewMembershipManager(node)

// 添加成员
err := membershipManager.AddMember(2, "192.168.1.2:8080")

// 移除成员
err := membershipManager.RemoveMember(2)

// 获取成员信息
members := membershipManager.GetMembers()
for _, member := range members {
    fmt.Printf("Node %d: %s (%s)\n", member.ID, member.Address, member.Status)
}
```

### 4. 性能优化

#### PerformanceOptimizer

多种性能优化功能：

```go
perfOptimizer := raft.NewPerformanceOptimizer(node)

// 启用批处理
batchConfig := raft.BatchConfig{
    Enabled:  true,
    MaxSize:  10,                    // 最大批次大小
    MaxDelay: 10 * time.Millisecond, // 最大延迟
    MaxBytes: 1024,                  // 最大字节数
}
perfOptimizer.EnableBatching(batchConfig)

// 启用流水线
pipelineConfig := raft.PipelineConfig{
    Enabled:        true,
    MaxInflight:    32,   // 最大并发请求数
    WindowSize:     100,  // 窗口大小
    EnableParallel: true, // 启用并行处理
}
perfOptimizer.EnablePipelining(pipelineConfig)

// 启用预投票
perfOptimizer.EnablePreVote(true)
```

### 5. ReadIndex/LeaseRead

线性化读取优化：

```go
// 配置 ReadIndex
config := raft.DefaultConfig()
config.ReadOnlyOption = raft.ReadOnlySafe // 启用 ReadIndex
config.LeaseTimeout = 5 * time.Second     // 设置租约超时

// 执行线性化读取
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

readIndex, err := node.LinearizableRead(ctx)
if err != nil {
    log.Printf("ReadIndex failed: %v", err)
} else {
    log.Printf("ReadIndex successful, read index: %d", readIndex)
}
```

### 6. Learner 节点支持

非投票成员支持：

```go
// 配置 Learner 节点
config := raft.DefaultConfig()
config.IsLearner = true  // 设置为 Learner 节点

// Learner 节点管理
learnerManager := raft.NewLearnerManager()

// 添加 Learner
err := learnerManager.AddLearner(nodeID, address)

// 提升 Learner 为正式成员
err := learnerManager.PromoteLearner(nodeID)

// 移除 Learner
err := learnerManager.RemoveLearner(nodeID)
```

### 7. 网络分区处理

PreVote 和分区检测：

```go
// 启用 PreVote 和法定人数检查
config := raft.DefaultConfig()
config.PreVote = true      // 启用 PreVote
config.CheckQuorum = true  // 启用法定人数检查

// PreVote 管理器
preVoteManager := raft.NewPreVoteManager(true, true)

// 网络分区检测器
partitionDetector := raft.NewNetworkPartitionDetector(true, config.ElectionTimeout*3)

// 领导权转移
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := node.TransferLeadership(ctx, targetNodeID)
if err != nil {
    log.Printf("领导权转移失败: %v", err)
}
```

### 8. 事件驱动日志系统

基于 Uber fx 设计思路的事件驱动日志系统：

#### 核心设计特性

- **事件驱动架构**：将 Raft 操作转换为结构化事件
- **类型安全**：强类型事件定义和接口驱动设计
- **可扩展性**：支持自定义事件类型和日志器实现
- **解耦合**：事件发射与日志处理完全分离
- **结构化**：统一的事件格式和字段命名

#### 事件系统

支持 15+ 种事件类型，覆盖 Raft 的所有关键操作：

```go
// 事件类型示例
type Event interface {
    GetType() EventType
    GetTimestamp() time.Time
    GetNodeID() uint64
}

// 支持的事件类型
const (
    EventNodeStarting    = "node_starting"
    EventNodeStarted     = "node_started"
    EventStateChanged    = "state_changed"
    EventElectionStarted = "election_started"
    EventLogAppended     = "log_appended"
    EventMessageSent     = "message_sent"
    // ... 更多事件类型
)
```

#### 多种日志器实现

```go
// 1. 控制台日志器 - 开发调试
consoleLogger := logging.NewConsoleLogger(logging.LevelInfo)

// 2. 结构化日志器 - 生产环境
slogLogger := logging.NewSlogLogger(logging.LevelInfo)

// 3. 多日志器组合 - 同时输出到多个目标
multiLogger := logging.NewMultiLogger(consoleLogger, slogLogger)

// 4. 空日志器 - 性能测试
nopLogger := logging.NewNopLogger()
```

#### 事件发射器

提供便利的事件发射方法：

```go
emitter := logging.NewEventEmitter(logger, nodeID)

// 节点生命周期事件
emitter.EmitNodeStarting(config)
emitter.EmitNodeStarted()
emitter.EmitNodeStopping()

// 状态变更事件
emitter.EmitStateChanged("follower", "candidate")
emitter.EmitTermChanged(1, 2)
emitter.EmitLeaderChanged(0, 1)

// 选举事件
emitter.EmitElectionStarted(term)
emitter.EmitElectionWon(term)
emitter.EmitVoteGranted(candidateID, term)

// 日志操作事件
emitter.EmitLogAppending(count, firstIndex)
emitter.EmitLogAppended(count, firstIndex, lastIndex, duration, err)
emitter.EmitLogCommitting(index)
emitter.EmitLogCommitted(index, duration, err)

// 消息传输事件
emitter.EmitMessageSending(to, msgType)
emitter.EmitMessageSent(to, msgType, duration, err)
emitter.EmitMessageReceived(from, msgType)

// 快照事件
emitter.EmitSnapshotCreating(index)
emitter.EmitSnapshotCreated(index, size, duration, err)

// 错误和警告
emitter.EmitError(operation, err)
emitter.EmitWarning(operation, message)
```

#### 配置系统

灵活的配置选项：

```go
// 基础配置
config := logging.LoggerConfig{
    Type:  logging.LoggerTypeSlog,
    Level: logging.LevelInfo,
    Output: logging.OutputConfig{
        Target: "file",
        File: &logging.FileConfig{
            Path: "raft.log",
        },
    },
    Format: logging.FormatConfig{
        JSON:      true,
        AddSource: true,
    },
}

// 使用工厂创建
factory := logging.NewLoggerFactory()
logger, err := factory.CreateLogger(config)

// 使用构建器
builder := logging.NewLoggerBuilder()
logger := builder.
    SetType(logging.LoggerTypeConsole).
    SetLevel(logging.LevelDebug).
    SetFormat(logging.FormatConfig{JSON: false}).
    Build()
```

#### 依赖注入集成

与依赖注入系统深度集成：

```go
// 应用构建器
builder := di.NewApplicationBuilder()

// 控制台日志
builder.WithConsoleLogging(logging.LevelInfo)

// JSON 日志到文件
builder.WithJSONLogging(logging.LevelDebug, "raft.log")

// 默认日志配置
builder.WithDefaultLogging()

// 构建应用
app, err := builder.Build()

// 获取事件发射器
emitter, err := app.GetNodeEventEmitter()

// 获取日志器
logger, err := app.GetLogger()
```

#### 性能特性

- **零分配优化**：事件对象复用，减少 GC 压力
- **级别过滤**：在发射阶段过滤，避免不必要的处理
- **异步支持**：可选的异步日志处理
- **批量优化**：支持批量日志写入

#### 扩展性

支持自定义扩展：

```go
// 自定义事件类型
type CustomEvent struct {
    *logging.BaseEvent
    CustomField string
}

func (e *CustomEvent) GetType() logging.EventType {
    return "custom_event"
}

// 自定义日志器
type CustomLogger struct {
    level logging.Level
}

func (l *CustomLogger) LogEvent(event logging.Event) {
    // 自定义处理逻辑
}

// 自定义发射器
type CustomEmitter struct {
    *logging.DefaultEventEmitter
}

func (e *CustomEmitter) EmitCustomEvent(data string) {
    event := &CustomEvent{
        BaseEvent: logging.NewBaseEvent("custom", e.GetNodeID()),
        CustomField: data,
    }
    e.EmitEvent(event)
}
```

## 生产示例

### 完整的生产级配置

```go
// 1. 使用依赖注入构建完整应用
builder := di.NewApplicationBuilder()

// 配置日志系统
builder.WithJSONLogging(logging.LevelInfo, "raft.log")

// 配置 Raft 节点
builder.WithRaft(nodeID)

// 构建应用
app, err := builder.Build()
if err != nil {
    log.Fatal(err)
}

// 获取组件
node, err := app.GetRaftNode()
if err != nil {
    log.Fatal(err)
}

emitter, err := app.GetNodeEventEmitter()
if err != nil {
    log.Fatal(err)
}

// 2. 手动配置方式（更多控制）
// 创建生产配置
config := raft.DefaultConfig()
config.ID = 1
config.SnapshotThreshold = 1000
config.CompactThreshold = 500
config.ReadOnlyOption = raft.ReadOnlySafe
config.LeaseTimeout = 5 * time.Second
config.PreVote = true
config.CheckQuorum = true

// 创建文件存储
storage, err := raft.NewFileStorage("./data")
if err != nil {
    log.Fatal(err)
}
config.Storage = storage

// 配置日志系统
loggerConfig := logging.LoggerConfig{
    Type:  logging.LoggerTypeSlog,
    Level: logging.LevelInfo,
    Output: logging.OutputConfig{
        Target: "file",
        File: &logging.FileConfig{
            Path: "raft.log",
        },
    },
    Format: logging.FormatConfig{
        JSON:      true,
        AddSource: true,
    },
}

factory := logging.NewLoggerFactory()
logger, err := factory.CreateLogger(loggerConfig)
if err != nil {
    log.Fatal(err)
}

// 创建事件发射器
emitter := logging.NewEventEmitter(logger, config.ID)

// 创建节点
node := raft.NewNode(config, stateMachine)

// 设置事件发射器到节点（如果支持）
if nodeWithLogging, ok := node.(interface{ SetEventEmitter(logging.EventEmitter) }); ok {
    nodeWithLogging.SetEventEmitter(emitter)
}

// 创建管理器
snapshotManager := raft.NewSnapshotManager(node, storage, stateMachine)
membershipManager := raft.NewMembershipManager(node)
perfOptimizer := raft.NewPerformanceOptimizer(node)

// 配置性能优化
perfOptimizer.EnableBatching(raft.BatchConfig{
    Enabled:  true,
    MaxSize:  10,
    MaxDelay: 10 * time.Millisecond,
    MaxBytes: 1024,
})

perfOptimizer.EnablePipelining(raft.PipelineConfig{
    Enabled:        true,
    MaxInflight:    32,
    WindowSize:     100,
    EnableParallel: true,
})

perfOptimizer.EnablePreVote(true)

// 发射启动事件
emitter.EmitNodeStarting(map[string]interface{}{
    "node_id": config.ID,
    "config":  "production",
})

// 启动所有组件
node.Start()
snapshotManager.Start()

emitter.EmitNodeStarted()

// 优雅关闭
defer func() {
    emitter.EmitNodeStopping()
    node.Stop()
    snapshotManager.Stop()
    emitter.Close()
}()
```

### 生产示例程序

运行完整的生产级示例：

```bash
# 运行生产示例
cd examples/production
go run main.go
```

提供的 HTTP API：
- `PUT /kv/{key}` - 设置键值
- `GET /kv/{key}` - 获取键值
- `GET /stats` - 获取性能统计
- `POST /snapshot` - 手动创建快照

## Performance Features

### 基础性能特性
- **高并发**: 使用 Go 的 goroutines 和 channels 实现
- **低延迟**: 优化的消息处理和批处理机制
- **可扩展**: 模块化设计支持自定义存储和传输层
- **内存高效**: 合理的内存管理和垃圾回收

### 高级性能优化
- **批处理**: 将多个小请求合并为批次处理，减少网络开销
- **流水线**: 并行发送多个 AppendEntries 请求，提高吞吐量
- **预投票**: 减少不必要的选举，提高系统稳定性
- **ReadIndex**: 线性化读取优化，提高读取性能
- **快照压缩**: 自动日志压缩，减少内存和存储使用

### 性能指标
- **吞吐量**: 每秒处理的操作数
- **延迟**: 操作完成时间
- **提交率**: 成功提交的操作比例
- **批处理效率**: 批处理的平均大小

## Testing

项目包含全面的单元测试和集成测试：

### 基础测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定包的测试
go test -v ./raft

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 第二阶段功能测试

```bash
# 运行第二阶段功能测试
go test ./raft -v

# 运行特定功能测试
go test ./raft -run TestFileStorage -v
go test ./raft -run TestSnapshotManager -v
go test ./raft -run TestMembershipManager -v
go test ./raft -run TestPerformanceOptimizer -v
go test ./raft -run TestIntegration -v
```

### 测试覆盖

#### 第一阶段测试 ✅
- ✅ 基础功能测试
- ✅ 选举测试
- ✅ 日志复制测试
- ✅ 状态转换测试

#### 第二阶段测试 ✅
- ✅ **TestFileStorage**: 文件存储功能测试
- ✅ **TestSnapshotManager**: 快照管理器测试
- ✅ **TestMembershipManager**: 成员管理功能测试
- ✅ **TestPerformanceOptimizer**: 性能优化功能测试
- ✅ **TestBatchEncoding**: 批处理编码功能测试
- ✅ **TestIntegration**: 集成测试

### 性能测试

```bash
# 运行性能基准测试
go test -bench=. ./raft

# 运行内存分析
go test -memprofile=mem.prof ./raft
go tool pprof mem.prof

# 运行 CPU 分析
go test -cpuprofile=cpu.prof ./raft
go tool pprof cpu.prof
```

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the project
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Create a Pull Request

## 应用场景

### 适用场景
- **分布式数据库**：作为分布式数据库的一致性层
- **配置管理**：分布式配置中心的一致性保证
- **服务发现**：服务注册和发现的一致性存储
- **分布式锁**：实现分布式锁和协调服务
- **状态机复制**：任何需要状态机复制的场景

### 性能指标
- **吞吐量**：支持高并发的读写操作
- **延迟**：毫秒级的操作延迟
- **可用性**：99.9%+ 的系统可用性
- **一致性**：强一致性保证

## 部署建议

### 集群配置
- **节点数量**：建议 3-7 个节点的奇数集群
- **硬件要求**：SSD 存储，充足的内存和网络带宽
- **网络要求**：低延迟、高可靠的网络连接
- **监控**：部署监控系统跟踪性能指标

### 配置优化
- **快照阈值**：根据数据量调整快照创建频率
- **批处理**：根据负载特性调整批处理参数
- **网络**：优化网络缓冲区和超时设置
- **存储**：使用高性能存储设备

### 性能建议

#### 1. 存储优化
- 使用 SSD 存储以获得更好的 I/O 性能
- 定期清理旧的快照文件
- 考虑使用压缩来减少存储空间

#### 2. 快照优化
- 根据应用特性调整快照阈值
- 在低负载时期创建快照
- 使用增量快照（如果状态机支持）

#### 3. 网络优化
- 启用批处理以减少网络请求
- 使用流水线提高并发度
- 调整网络缓冲区大小

#### 4. 内存优化
- 及时压缩日志以释放内存
- 监控内存使用情况
- 使用对象池减少 GC 压力

## 监控和调试

### 内置监控

```go
// 获取节点状态
state := node.State()
fmt.Printf("节点状态: %s\n", state)

// 获取集群信息
info := node.ClusterInfo()
fmt.Printf("集群大小: %d\n", info.Size)
fmt.Printf("当前任期: %d\n", info.Term)
fmt.Printf("领导者: %d\n", info.Leader)

// 获取性能指标
metrics := node.Metrics()
fmt.Printf("已提交日志: %d\n", metrics.CommittedEntries)
fmt.Printf("应用日志: %d\n", metrics.AppliedEntries)
```

### 事件驱动监控

```go
// 创建监控日志器
monitorConfig := logging.LoggerConfig{
    Type:  logging.LoggerTypeSlog,
    Level: logging.LevelDebug,
    Output: logging.OutputConfig{
        Target: "file",
        File: &logging.FileConfig{
            Path: "monitor.log",
        },
    },
    Format: logging.FormatConfig{
        JSON:      true,
        AddSource: true,
    },
}

factory := logging.NewLoggerFactory()
monitorLogger, err := factory.CreateLogger(monitorConfig)
if err != nil {
    log.Fatal(err)
}

// 创建监控事件发射器
monitor := logging.NewEventEmitter(monitorLogger, nodeID)

// 监控关键事件
monitor.EmitElectionStarted(map[string]interface{}{
    "reason": "heartbeat_timeout",
    "term":   currentTerm,
})

monitor.EmitVoteGranted(candidateID, currentTerm)

monitor.EmitLeaderElected(leaderID, currentTerm, map[string]interface{}{
    "election_duration": electionDuration,
    "votes_received":    votesReceived,
})

// 监控日志复制
monitor.EmitLogEntryReceived(entry.Index, entry.Term, len(entry.Data))
monitor.EmitLogEntryCommitted(entry.Index, entry.Term)
monitor.EmitLogEntryApplied(entry.Index, entry.Term, result)

// 监控快照操作
monitor.EmitSnapshotStarted(lastIndex, lastTerm)
monitor.EmitSnapshotCompleted(lastIndex, lastTerm, snapshotSize)

// 监控成员变更
monitor.EmitMemberAdded(newMemberID, map[string]interface{}{
    "address": newMemberAddress,
    "role":    "voter",
})

monitor.EmitMemberRemoved(removedMemberID, "graceful_shutdown")
```

### 调试工具

```go
// 1. 启用详细日志
debugConfig := logging.LoggerConfig{
    Type:  logging.LoggerTypeConsole,
    Level: logging.LevelDebug,
    Output: logging.OutputConfig{
        Target: "stderr",
    },
    Format: logging.FormatConfig{
        JSON:      false,
        AddSource: true,
    },
}

debugLogger, err := factory.CreateLogger(debugConfig)
if err != nil {
    log.Fatal(err)
}

debugEmitter := logging.NewEventEmitter(debugLogger, nodeID)

// 2. 多日志器组合（同时输出到控制台和文件）
multiLogger := logging.NewMultiLogger(
    consoleLogger,
    fileLogger,
    debugLogger,
)

combinedEmitter := logging.NewEventEmitter(multiLogger, nodeID)

// 3. 导出状态快照
snapshot, err := node.ExportSnapshot()
if err != nil {
    debugEmitter.EmitError("snapshot_export_failed", err, map[string]interface{}{
        "node_id": nodeID,
    })
    log.Fatal(err)
}

debugEmitter.EmitSnapshotCompleted(snapshot.LastIndex, snapshot.LastTerm, len(snapshot.Data))

// 4. 分析日志条目
entries, err := storage.Entries(1, 100)
if err != nil {
    debugEmitter.EmitError("entries_fetch_failed", err, map[string]interface{}{
        "start_index": 1,
        "end_index":   100,
    })
    log.Fatal(err)
}

for _, entry := range entries {
    debugEmitter.EmitLogEntryReceived(entry.Index, entry.Term, len(entry.Data))
    fmt.Printf("条目 %d: 任期=%d, 类型=%s\n", 
        entry.Index, entry.Term, entry.Type)
}

// 5. 实时事件监控
go func() {
    for {
        select {
        case event := <-eventChannel:
            debugEmitter.EmitCustomEvent("realtime_monitor", map[string]interface{}{
                "event_type": event.Type,
                "timestamp":  time.Now(),
                "data":       event.Data,
            })
        case <-ctx.Done():
            return
        }
    }
}()
```

### 日志分析工具

```go
// 解析和分析日志文件
func analyzeLogFile(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    stats := make(map[string]int)
    
    for scanner.Scan() {
        var logEntry map[string]interface{}
        if err := json.Unmarshal(scanner.Bytes(), &logEntry); err != nil {
            continue
        }
        
        if eventType, ok := logEntry["event_type"].(string); ok {
            stats[eventType]++
        }
    }
    
    fmt.Println("事件统计:")
    for eventType, count := range stats {
        fmt.Printf("  %s: %d\n", eventType, count)
    }
    
    return scanner.Err()
}

// 性能分析
func analyzePerformance(emitter logging.EventEmitter) {
    start := time.Now()
    
    // 执行操作...
    
    duration := time.Since(start)
    emitter.EmitCustomEvent("performance_analysis", map[string]interface{}{
        "operation":     "batch_commit",
        "duration_ms":   duration.Milliseconds(),
        "entries_count": entriesCount,
        "throughput":    float64(entriesCount) / duration.Seconds(),
    })
}
```

### 性能指标
- **吞吐量**：每秒处理的操作数
- **延迟**：操作完成时间
- **提交率**：成功提交的操作比例
- **批处理效率**：批处理的平均大小

### 日志记录
所有组件都提供详细的日志记录，可以通过调整日志级别来控制输出：

```go
// 传统方式设置日志级别
log.SetLevel(log.DebugLevel)

// 使用新的日志系统
config := logging.LoggerConfig{
    Type:  logging.LoggerTypeConsole,
    Level: logging.LevelDebug,
    Output: logging.OutputConfig{
        Target: "stderr",
    },
}

logger, err := logging.NewLoggerFactory().CreateLogger(config)
if err != nil {
    log.Fatal(err)
}

emitter := logging.NewEventEmitter(logger, nodeID)
```



## 日志系统总结

Go-Raft 的事件驱动日志系统为分布式系统提供了强大的可观测性支持：

### 核心优势
- **事件驱动**：基于事件的结构化日志，便于分析和监控
- **类型安全**：强类型事件定义，避免运行时错误
- **高性能**：异步处理，不影响核心 Raft 性能
- **可扩展**：支持自定义事件和第三方日志库集成
- **生产就绪**：完整的配置系统和依赖注入支持

### 适用场景
- **开发调试**：详细的事件追踪和状态监控
- **生产监控**：关键指标收集和异常告警
- **性能分析**：吞吐量、延迟等性能指标统计
- **故障排查**：完整的事件链路追踪
- **合规审计**：结构化的操作日志记录

### 集成建议
1. **开发环境**：使用控制台日志器，启用详细级别
2. **测试环境**：使用文件日志器，JSON 格式便于分析
3. **生产环境**：使用多日志器组合，同时输出到文件和监控系统
4. **微服务架构**：通过依赖注入统一配置日志系统
5. **云原生部署**：集成 Prometheus、ELK 等监控栈

Go-Raft 项目为分布式系统开发者提供了一个可靠、高效、易用的一致性算法实现，配备完整的事件驱动日志系统，可以作为构建分布式应用的重要基础组件。

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

