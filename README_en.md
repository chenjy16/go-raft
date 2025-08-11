# Go-Raft

A production-ready Raft consensus algorithm library implemented in Go.

## Project Overview

Go-Raft is a Raft distributed consensus algorithm library implemented in Go. The project is developed in two phases:

1. **Phase 1 (Basic Implementation)**: Implements the core functions of the Raft algorithm.
2. **Phase 2 (Production-Ready)**: Adds advanced features required for production environments.

## Features

### Phase 1 Features ✅
- [x] **Core Raft Algorithm**: Leader election, log replication, safety guarantees.
- [x] **In-Memory Storage Backend**: MemoryStorage implementation.
- [x] **Key-Value Store State Machine**: Simple KV store example.
- [x] **HTTP RPC Transport Layer**: Production network transport.
- [x] **In-Memory Transport Layer**: Test environment transport implementation.
- [x] **Basic Test Cases**: Core functionality test coverage.
- [x] **Complete Example Programs**: From simple to complex usage examples.

### Phase 2 Features (Production-Ready) ✅
- [x] **Persistent Storage Support**: FileStorage implementation.
- [x] **Snapshots and Log Compaction**: SnapshotManager for automatic snapshot management.
- [x] **Cluster Membership Management**: MembershipManager for dynamic membership changes.
- [x] **Performance Optimizations**: Batching, pipelining, pre-vote optimization.
- [x] **ReadIndex/LeaseRead**: Linearizable read optimizations.
- [x] **Learner Node Support**: Non-voting member support.
- [x] **Network Partition Handling**: PreVote and partition detection.
- [x] **Event-Driven Logging System**: Structured logging, multiple loggers, dependency injection.
- [x] **Production-Level Examples**: Complete production environment examples.
- [x] **Monitoring and Metrics**: Performance statistics and monitoring functions.

### Advanced Features ✅
- [x] **Strong Consistency Guarantees**: Complete Raft algorithm implementation.
- [x] **Fault Tolerance**: Supports minority node failures.
- [x] **Partition Tolerance**: Maintains availability during network partitions.
- [x] **High Performance**: Batching, pipelining, and other optimization techniques.
- [x] **Scalability**: Supports dynamic addition and removal of nodes.
- [x] **Production Features**: Persistence, snapshots, monitoring, etc.

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
├── raft/                    # Core Raft implementation
│   ├── types.go            # Basic type definitions
│   ├── node.go             # Raft node implementation
│   ├── log.go              # Log management
│   ├── storage.go          # Storage interface and memory implementation
│   ├── file_storage.go     # File storage implementation (Phase 2)
│   ├── snapshot.go         # Snapshot management (Phase 2)
│   ├── membership.go       # Membership management (Phase 2)
│   ├── performance.go      # Performance optimization (Phase 2)
│   ├── readindex.go        # ReadIndex implementation (Phase 2)
│   ├── learner.go          # Learner node support (Phase 2)
│   ├── prevote.go          # PreVote and network partition handling (Phase 2)
│   ├── errors.go           # Error definitions (Phase 2)
│   ├── logging/            # Event-driven logging system (Phase 2)
│   │   ├── README.md       # Logging system detailed documentation
│   │   ├── event.go        # Event system definitions
│   │   ├── logger.go       # Logger implementations
│   │   ├── emitter.go      # Event emitter
│   │   └── config.go       # Configuration and factory
│   ├── di/                 # Dependency injection system (Phase 2)
│   │   └── module.go       # DI container and modules
│   ├── raft_test.go        # Basic tests
│   └── stage2_test.go      # Phase 2 tests
├── transport/               # Network transport layer
│   └── transport.go        # HTTP RPC and memory transport implementations
├── examples/                # Example programs
│   ├── simple/             # Simple example
│   ├── kvstore/            # Key-value store state machine
│   ├── three-node-cluster/ # Three-node cluster example
│   ├── production/         # Production-level example (Phase 2)
│   └── advanced_features.go # Advanced features demonstration (Phase 2)
├── Makefile                # Build scripts
├── go.mod                  # Go module definitions
├── README.md               # Project documentation
├── LOGGING_IMPLEMENTATION.md # Logging system implementation summary
├── STAGE2_README.md        # Phase 2 features description
├── STAGE2_COMPLETION.md    # Phase 2 completion report
└── PROJECT_SUMMARY.md      # Project summary
```

### Core Components

#### 1. Raft Node (`raft.Node`)
- Implements complete Raft algorithm logic.
- Supports leader election, log replication, and safety guarantees.
- Provides asynchronous message handling mechanism.
- Integrates advanced features like ReadIndex, Learner, PreVote.

#### 2. Storage Interface (`raft.Storage`)
- Abstract storage interface supporting multiple backends.
- Memory storage implementation (`MemoryStorage`).
- File storage implementation (`FileStorage`) - for production persistence.
- Supports persistence of log entries, hard state, and snapshots.

#### 3. Transport Layer (`transport.Transport`)
- HTTP RPC transport (production).
- Memory transport (testing).
- Supports reliable message delivery.

#### 4. State Machine Interface (`raft.StateMachine`)
- User-defined business logic.
- Supports snapshots and recovery.
- Example: Key-value store state machine.

#### 5. Advanced Managers (Phase 2)
- **SnapshotManager**: Automatic snapshot management and log compaction.
- **MembershipManager**: Dynamic cluster membership management.
- **PerformanceOptimizer**: Performance optimizations (batching, pipelining).
- **ReadIndexManager**: Linearizable read optimizations.
- **LearnerManager**: Learner node management.
- **PreVoteManager**: PreVote and network partition handling.

#### 6. Logging System (Event-Driven Logging System)
- **Event System**: Structured event definitions, supporting 15+ event types.
- **Logger Interface**: Unified logger interface, supporting multiple implementations.
- **EventEmitter**: Event emitter providing convenient event emission methods.
- **Multiple Loggers**: ConsoleLogger, SlogLogger, MultiLogger, NopLogger.
- **Configuration**: Flexible configuration system supporting multiple output formats and targets.

#### 7. Dependency Injection (Dependency Injection System)
- **Container**: Lightweight DI container with type-safe service registration and resolution.
- **ApplicationBuilder**: Application builder supporting chained configuration.
- **Module System**: Modular configuration supporting logging and Raft modules.
- **Integration**: Deep integration with the logging system for simplified configuration.

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
    // 1. Directly use logger
    logger := logging.NewConsoleLogger(logging.LevelInfo)
    emitter := logging.NewEventEmitter(logger, 1) // nodeID = 1
    
    // Emit event
    emitter.EmitNodeStarting(map[string]interface{}{
        "config": "default",
    })
    
    // 2. Create logger with configuration
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
    
    // 3. Use dependency injection
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
    
    // Use event emitter
    emitter.EmitStateChanged("follower", "candidate")
    emitter.EmitElectionStarted(1)
    emitter.EmitLogAppended(10, 1, 10, time.Millisecond*5, nil)
}
```

#### Advanced Logging Configuration

```go
// Multi-logger combination
consoleLogger := logging.NewConsoleLogger(logging.LevelInfo)
jsonLogger := logging.NewSlogLogger(logging.LevelDebug)
multiLogger := logging.NewMultiLogger(consoleLogger, jsonLogger)

// Use builder configuration
builder := logging.NewLoggerBuilder()
logger := builder.
    SetType(logging.LoggerTypeSlog).
    SetLevel(logging.LevelInfo).
    SetFormat(logging.FormatConfig{JSON: true}).
    SetOutput(logging.OutputConfig{Target: "file", File: &logging.FileConfig{Path: "raft.log"}}).
    Build()

// Dependency injection configuration
appBuilder := di.NewApplicationBuilder()
appBuilder.WithJSONLogging(logging.LevelDebug, "raft.log")
appBuilder.WithRaft(nodeID)

app, _ := appBuilder.Build()
```

## Phase 2 Features Details

### 1. Persistent Storage Support

#### FileStorage

`FileStorage` provides a file system-based persistent storage implementation:

```go
// Create file storage
storage, err := raft.NewFileStorage("/path/to/data")
if err != nil {
    log.Fatal(err)
}
defer storage.Close()

// Use in configuration
config := raft.DefaultConfig()
config.Storage = storage
```

**File Structure:**
```
data/
├── hard_state.json    # Hard state
├── entries.log        # Log entries
└── snapshot.snap      # Snapshot data
```

### 2. Snapshots and Log Compaction

#### SnapshotManager

Automatic snapshot management features:

```go
// Create snapshot manager
snapshotManager := raft.NewSnapshotManager(node, storage, stateMachine)

// Set snapshot thresholds
snapshotManager.SetSnapshotThreshold(1000)  // Create snapshot after 1000 entries
snapshotManager.SetCompactThreshold(500)    // Retain last 500 entries

// Start manager
snapshotManager.Start()
defer snapshotManager.Stop()

// Manually create snapshot
err := snapshotManager.CreateSnapshotNow()
```

### 3. Cluster Membership Management

#### MembershipManager

Dynamic cluster membership management:

```go
// Create membership manager
membershipManager := raft.NewMembershipManager(node)

// Add member
err := membershipManager.AddMember(2, "192.168.1.2:8080")

// Remove member
err := membershipManager.RemoveMember(2)

// Get member information
members := membershipManager.GetMembers()
for _, member := range members {
    fmt.Printf("Node %d: %s (%s)\n", member.ID, member.Address, member.Status)
}
```

### 4. Performance Optimizations

#### PerformanceOptimizer

Multiple performance optimization features:

```go
perfOptimizer := raft.NewPerformanceOptimizer(node)

// Enable batching
batchConfig := raft.BatchConfig{
    Enabled:  true,
    MaxSize:  10,                    // Max batch size
    MaxDelay: 10 * time.Millisecond, // Max delay
    MaxBytes: 1024,                  // Max bytes
}
perfOptimizer.EnableBatching(batchConfig)

// Enable pipelining
pipelineConfig := raft.PipelineConfig{
    Enabled:        true,
    MaxInflight:    32,   // Max concurrent requests
    WindowSize:     100,  // Window size
    EnableParallel: true, // Enable parallel processing
}
perfOptimizer.EnablePipelining(pipelineConfig)

// Enable pre-vote
perfOptimizer.EnablePreVote(true)
```

### 5. ReadIndex/LeaseRead

Linearizable read optimizations:

```go
// Configure ReadIndex
config := raft.DefaultConfig()
config.ReadOnlyOption = raft.ReadOnlySafe // Enable ReadIndex
config.LeaseTimeout = 5 * time.Second     // Set lease timeout

// Perform linearizable read
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

readIndex, err := node.LinearizableRead(ctx)
if err != nil {
    log.Printf("ReadIndex failed: %v", err)
} else {
    log.Printf("ReadIndex successful, read index: %d", readIndex)
}
```

### 6. Learner Node Support

Non-voting member support:

```go
// Configure Learner node
config := raft.DefaultConfig()
config.IsLearner = true  // Set as Learner node

// Learner node management
learnerManager := raft.NewLearnerManager()

// Add Learner
err := learnerManager.AddLearner(nodeID, address)

// Promote Learner to full member
err := learnerManager.PromoteLearner(nodeID)

// Remove Learner
err := learnerManager.RemoveLearner(nodeID)
```

### 7. Network Partition Handling

PreVote and partition detection:

```go
// Enable PreVote and quorum check
config := raft.DefaultConfig()
config.PreVote = true      // Enable PreVote
config.CheckQuorum = true  // Enable quorum check

// PreVote manager
preVoteManager := raft.NewPreVoteManager(true, true)

// Network partition detector
partitionDetector := raft.NewNetworkPartitionDetector(true, config.ElectionTimeout*3)

// Leadership transfer
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := node.TransferLeadership(ctx, targetNodeID)
if err != nil {
    log.Printf("Leadership transfer failed: %v", err)
}
```

### 8. Event-Driven Logging System

Event-driven logging system based on Uber fx design ideas:

#### Core Design Features
- **Event-Driven Architecture**: Converts Raft operations to structured events.
- **Type Safety**: Strongly typed event definitions and interface-driven design.
- **Extensibility**: Supports custom event types and logger implementations.
- **Decoupling**: Complete separation of event emission and log handling.
- **Structured**: Unified event format and field naming.

#### Event System
Supports 15+ event types covering all key Raft operations:

```go
// Event type example
type Event interface {
    GetType() EventType
    GetTimestamp() time.Time
    GetNodeID() uint64
}

// Supported event types
const (
    EventNodeStarting    = "node_starting"
    EventNodeStarted     = "node_started"
    EventStateChanged    = "state_changed"
    EventElectionStarted = "election_started"
    EventLogAppended     = "log_appended"
    EventMessageSent     = "message_sent"
    // ... more event types
)
```

#### Multiple Logger Implementations

```go
// 1. Console logger - for development debugging
consoleLogger := logging.NewConsoleLogger(logging.LevelInfo)

// 2. Structured logger - for production
slogLogger := logging.NewSlogLogger(logging.LevelInfo)

// 3. Multi-logger combination - output to multiple targets
multiLogger := logging.NewMultiLogger(consoleLogger, slogLogger)

// 4. Nop logger - for performance testing
nopLogger := logging.NewNopLogger()
```

#### Event Emitter
Provides convenient event emission methods:

```go
emitter := logging.NewEventEmitter(logger, nodeID)

// Node lifecycle events
emitter.EmitNodeStarting(config)
emitter.EmitNodeStarted()
emitter.EmitNodeStopping()

// State change events
emitter.EmitStateChanged("follower", "candidate")
emitter.EmitTermChanged(1, 2)
emitter.EmitLeaderChanged(0, 1)

// Election events
emitter.EmitElectionStarted(term)
emitter.EmitElectionWon(term)
emitter.EmitVoteGranted(candidateID, term)

// Log operation events
emitter.EmitLogAppending(count, firstIndex)
emitter.EmitLogAppended(count, firstIndex, lastIndex, duration, err)
emitter.EmitLogCommitting(index)
emitter.EmitLogCommitted(index, duration, err)

// Message transmission events
emitter.EmitMessageSending(to, msgType)
emitter.EmitMessageSent(to, msgType, duration, err)
emitter.EmitMessageReceived(from, msgType)

// Snapshot events
emitter.EmitSnapshotCreating(index)
emitter.EmitSnapshotCreated(index, size, duration, err)

// Error and warning
emitter.EmitError(operation, err)
emitter.EmitWarning(operation, message)
```

#### Configuration System
Flexible configuration options:

```go
// Basic configuration
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

// Create using factory
factory := logging.NewLoggerFactory()
logger, err := factory.CreateLogger(config)

// Use builder
builder := logging.NewLoggerBuilder()
logger := builder.
    SetType(logging.LoggerTypeConsole).
    SetLevel(logging.LevelDebug).
    SetFormat(logging.FormatConfig{JSON: false}).
    Build()
```

#### Dependency Injection Integration
Deep integration with dependency injection system:

```go
// Application builder
builder := di.NewApplicationBuilder()

// Console logging
builder.WithConsoleLogging(logging.LevelInfo)

// JSON logging to file
builder.WithJSONLogging(logging.LevelDebug, "raft.log")

// Default logging configuration
builder.WithDefaultLogging()

// Build application
app, err := builder.Build()

// Get event emitter
emitter, err := app.GetNodeEventEmitter()

// Get logger
logger, err := app.GetLogger()
```

#### Performance Features
- **Zero Allocation Optimization**: Event object reuse to reduce GC pressure.
- **Level Filtering**: Filter at emission stage to avoid unnecessary processing.
- **Asynchronous Support**: Optional asynchronous log handling.
- **Batch Optimization**: Supports batch log writes.

#### Extensibility
Supports custom extensions:

```go
// Custom event type
type CustomEvent struct {
    *logging.BaseEvent
    CustomField string
}

func (e *CustomEvent) GetType() logging.EventType {
    return "custom_event"
}

// Custom logger
type CustomLogger struct {
    level logging.Level
}

func (l *CustomLogger) LogEvent(event logging.Event) {
    // Custom handling logic
}

// Custom emitter
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

## Production Examples

### Complete Production-Level Configuration

```go
// 1. Build complete application using dependency injection
builder := di.NewApplicationBuilder()

// Configure logging system
builder.WithJSONLogging(logging.LevelInfo, "raft.log")

// Configure Raft node
builder.WithRaft(nodeID)

// Build application
app, err := builder.Build()
if err != nil {
    log.Fatal(err)
}

// Get components
node, err := app.GetRaftNode()
if err != nil {
    log.Fatal(err)
}

emitter, err := app.GetNodeEventEmitter()
if err != nil {
    log.Fatal(err)
}

// 2. Manual configuration (more control)
// Create production configuration
config := raft.DefaultConfig()
config.ID = 1
config.SnapshotThreshold = 1000
config.CompactThreshold = 500
config.ReadOnlyOption = raft.ReadOnlySafe
config.LeaseTimeout = 5 * time.Second
config.PreVote = true
config.CheckQuorum = true

// Create file storage
storage, err := raft.NewFileStorage("./data")
if err != nil {
    log.Fatal(err)
}
config.Storage = storage

// Configure logging system
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

// Create event emitter
emitter := logging.NewEventEmitter(logger, config.ID)

// Create node
node := raft.NewNode(config, stateMachine)

// Set event emitter to node (if supported)
if nodeWithLogging, ok := node.(interface{ SetEventEmitter(logging.EventEmitter) }); ok {
    nodeWithLogging.SetEventEmitter(emitter)
}

// Create managers
snapshotManager := raft.NewSnapshotManager(node, storage, stateMachine)
membershipManager := raft.NewMembershipManager(node)
perfOptimizer := raft.NewPerformanceOptimizer(node)

// Configure performance optimizations
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

// Emit start event
emitter.EmitNodeStarting(map[string]interface{}{
    "node_id": config.ID,
    "config":  "production",
})

// Start all components
node.Start()
snapshotManager.Start()

emitter.EmitNodeStarted()

// Graceful shutdown
defer func() {
    emitter.EmitNodeStopping()
    node.Stop()
    snapshotManager.Stop()
    emitter.Close()
}()
```

### Production Example Program

Run the complete production-level example:

```bash
# Run production example
cd examples/production
go run main.go
```

Provided HTTP APIs:
- `PUT /kv/{key}` - Set key-value
- `GET /kv/{key}` - Get key-value
- `GET /stats` - Get performance statistics
- `POST /snapshot` - Manually create snapshot

## Performance Features

### Basic Performance Characteristics
- **High Concurrency**: Implemented using Go's goroutines and channels.
- **Low Latency**: Optimized message handling and batching mechanisms.
- **Scalable**: Modular design supporting custom storage and transport layers.
- **Memory Efficient**: Reasonable memory management and garbage collection.

### Advanced Performance Optimizations
- **Batching**: Combines multiple small requests to reduce network overhead.
- **Pipelining**: Sends multiple AppendEntries requests in parallel to increase throughput.
- **Pre-Voting**: Reduces unnecessary elections to improve system stability.
- **ReadIndex**: Optimizes linearizable reads for better read performance.
- **Snapshot Compaction**: Automatic log compaction to reduce memory and storage usage.

### Performance Metrics
- **Throughput**: Number of operations processed per second.
- **Latency**: Operation completion time.
- **Commit Rate**: Proportion of successfully committed operations.
- **Batch Efficiency**: Average size of batches.

## Testing

The project includes comprehensive unit and integration tests:

### Basic Tests

```bash
# Run all tests
go test -v ./...

# Run tests for specific package
go test -v ./raft

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Phase 2 Feature Tests

```bash
# Run Phase 2 feature tests
go test ./raft -v

# Run specific feature tests
go test ./raft -run TestFileStorage -v
go test ./raft -run TestSnapshotManager -v
go test ./raft -run TestMembershipManager -v
go test ./raft -run TestPerformanceOptimizer -v
go test ./raft -run TestIntegration -v
```

### Test Coverage

#### Phase 1 Tests ✅
- ✅ Basic functionality tests
- ✅ Election tests
- ✅ Log replication tests
- ✅ State transition tests

#### Phase 2 Tests ✅
- ✅ **TestFileStorage**: File storage functionality tests
- ✅ **TestSnapshotManager**: Snapshot manager tests
- ✅ **TestMembershipManager**: Membership management functionality tests
- ✅ **TestPerformanceOptimizer**: Performance optimization functionality tests
- ✅ **TestBatchEncoding**: Batch encoding functionality tests
- ✅ **TestIntegration**: Integration tests

### Performance Tests

```bash
# Run performance benchmarks
go test -bench=. ./raft

# Run memory analysis
go test -memprofile=mem.prof ./raft
go tool pprof mem.prof

# Run CPU analysis
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

## Application Scenarios

### Suitable Scenarios
- **Distributed Databases**: As a consistency layer for distributed databases.
- **Configuration Management**: Consistency guarantees for distributed configuration centers.
- **Service Discovery**: Consistency storage for service registration and discovery.
- **Distributed Locks**: Implementing distributed locks and coordination services.
- **State Machine Replication**: Any scenario requiring state machine replication.

### Performance Metrics
- **Throughput**: Supports high-concurrency read/write operations.
- **Latency**: Millisecond-level operation latency.
- **Availability**: 99.9%+ system availability.
- **Consistency**: Strong consistency guarantees.

## Deployment Recommendations

### Cluster Configuration
- **Number of Nodes**: Recommend 3-7 nodes in an odd-numbered cluster.
- **Hardware Requirements**: SSD storage, sufficient memory and network bandwidth.
- **Network Requirements**: Low-latency, high-reliability network connections.
- **Monitoring**: Deploy monitoring systems to track performance metrics.

### Configuration Optimizations
- **Snapshot Threshold**: Adjust snapshot creation frequency based on data volume.
- **Batching**: Adjust batching parameters based on load characteristics.
- **Network**: Optimize network buffers and timeout settings.
- **Storage**: Use high-performance storage devices.

### Performance Recommendations

#### 1. Storage Optimization
- Use SSD storage for better I/O performance.
- Regularly clean old snapshot files.
- Consider compression to reduce storage space.

#### 2. Snapshot Optimization
- Adjust snapshot threshold based on application characteristics.
- Create snapshots during low-load periods.
- Use incremental snapshots (if state machine supports).

#### 3. Network Optimization
- Enable batching to reduce network requests.
- Use pipelining to increase concurrency.
- Adjust network buffer sizes.

#### 4. Memory Optimization
- Compact logs timely to release memory.
- Monitor memory usage.
- Use object pools to reduce GC pressure.

## Monitoring and Debugging

### Built-in Monitoring

```go
// Get node state
state := node.State()
fmt.Printf("Node state: %s\n", state)

// Get cluster information
info := node.ClusterInfo()
fmt.Printf("Cluster size: %d\n", info.Size)
fmt.Printf("Current term: %d\n", info.Term)
fmt.Printf("Leader: %d\n", info.Leader)

// Get performance metrics
metrics := node.Metrics()
fmt.Printf("Committed logs: %d\n", metrics.CommittedEntries)
fmt.Printf("Applied logs: %d\n", metrics.AppliedEntries)
```

### Event-Driven Monitoring

```go
// Create monitoring logger
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

// Create monitoring event emitter
monitor := logging.NewEventEmitter(monitorLogger, nodeID)

// Monitor key events
monitor.EmitElectionStarted(map[string]interface{}{
    "reason": "heartbeat_timeout",
    "term":   currentTerm,
})

monitor.EmitVoteGranted(candidateID, currentTerm)

monitor.EmitLeaderElected(leaderID, currentTerm, map[string]interface{}{
    "election_duration": electionDuration,
    "votes_received":    votesReceived,
})

// Monitor log replication
monitor.EmitLogEntryReceived(entry.Index, entry.Term, len(entry.Data))
monitor.EmitLogEntryCommitted(entry.Index, entry.Term)
monitor.EmitLogEntryApplied(entry.Index, entry.Term, result)

// Monitor snapshot operations
monitor.EmitSnapshotStarted(lastIndex, lastTerm)
monitor.EmitSnapshotCompleted(lastIndex, lastTerm, snapshotSize)

// Monitor membership changes
monitor.EmitMemberAdded(newMemberID, map[string]interface{}{
    "address": newMemberAddress,
    "role":    "voter",
})

monitor.EmitMemberRemoved(removedMemberID, "graceful_shutdown")
```

### Debugging Tools

```go
// 1. Enable detailed logging
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

// 2. Multi-logger combination (output to console and file)
multiLogger := logging.NewMultiLogger(
    consoleLogger,
    fileLogger,
    debugLogger,
)

combinedEmitter := logging.NewEventEmitter(multiLogger, nodeID)

// 3. Export state snapshot
snapshot, err := node.ExportSnapshot()
if err != nil {
    debugEmitter.EmitError("snapshot_export_failed", err, map[string]interface{}{
        "node_id": nodeID,
    })
    log.Fatal(err)
}

debugEmitter.EmitSnapshotCompleted(snapshot.LastIndex, snapshot.LastTerm, len(snapshot.Data))

// 4. Analyze log entries
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
    fmt.Printf("Entry %d: Term=%d, Type=%s\n", 
        entry.Index, entry.Term, entry.Type)
}

// 5. Real-time event monitoring
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

### Log Analysis Tools

```go
// Parse and analyze log file
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
    
    fmt.Println("Event statistics:")
    for eventType, count := range stats {
        fmt.Printf("  %s: %d\n", eventType, count)
    }
    
    return scanner.Err()
}

// Performance analysis
func analyzePerformance(emitter logging.EventEmitter) {
    start := time.Now()
    
    // Perform operations...
    
    duration := time.Since(start)
    emitter.EmitCustomEvent("performance_analysis", map[string]interface{}{
        "operation":     "batch_commit",
        "duration_ms":   duration.Milliseconds(),
        "entries_count": entriesCount,
        "throughput":    float64(entriesCount) / duration.Seconds(),
    })
}
```

### Performance Metrics
- **Throughput**: Number of operations processed per second.
- **Latency**: Operation completion time.
- **Commit Rate**: Proportion of successfully committed operations.
- **Batch Efficiency**: Average size of batches.

### Logging
All components provide detailed logging, with output controlled by adjusting log levels:

```go
// Traditional way to set log level
log.SetLevel(log.DebugLevel)

// Using new logging system
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


## Logging System Summary

Go-Raft's event-driven logging system provides powerful observability support for distributed systems:

### Core Advantages
- **Event-Driven**: Structured event-based logging for easy analysis and monitoring.
- **Type Safety**: Strongly typed event definitions to avoid runtime errors.
- **High Performance**: Asynchronous processing without impacting core Raft performance.
- **Extensible**: Supports custom events and integration with third-party logging libraries.
- **Production-Ready**: Complete configuration system and dependency injection support.

### Applicable Scenarios
- **Development Debugging**: Detailed event tracking and state monitoring.
- **Production Monitoring**: Key metric collection and anomaly alerting.
- **Performance Analysis**: Throughput, latency, and other performance metric statistics.
- **Troubleshooting**: Complete event chain tracing.
- **Compliance Auditing**: Structured operation log recording.

### Integration Recommendations
1. **Development Environment**: Use console logger with detailed level.
2. **Testing Environment**: Use file logger with JSON format for analysis.
3. **Production Environment**: Use multi-logger combination, outputting to files and monitoring systems.
4. **Microservices Architecture**: Unify logging configuration via dependency injection.
5. **Cloud-Native Deployment**: Integrate with Prometheus, ELK, etc., monitoring stacks.

Go-Raft project provides a reliable, efficient, and easy-to-use consistency algorithm implementation for distributed system developers, equipped with a complete event-driven logging system, serving as an important foundational component for building distributed applications.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.