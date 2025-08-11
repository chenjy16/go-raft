# Raft 日志系统

基于 Uber fx 设计思路实现的事件驱动日志系统，具备解耦合、可扩展、类型安全、结构化和灵活配置的特性。

## 核心设计

### 1. 事件驱动架构

系统将 Raft 框架的各种操作转换为结构化事件，通过统一的 `Logger` 接口处理：

```go
type Logger interface {
    LogEvent(event Event)
    SetLevel(level Level)
    GetLevel() Level
    WithFields(fields map[string]interface{}) Logger
}
```

### 2. 事件联合类型

使用 Go 的接口和类型断言实现类似联合类型的设计：

```go
type Event interface {
    GetType() EventType
    GetTimestamp() time.Time
    GetNodeID() uint64
}
```

支持的事件类型包括：
- **节点生命周期事件**：启动、停止、状态变更
- **日志操作事件**：追加、提交、复制
- **消息传输事件**：发送、接收、处理
- **选举事件**：开始选举、投票、选举结果
- **快照事件**：创建、恢复、传输
- **错误和警告事件**：异常处理、性能监控

### 3. 多种日志器实现

#### ConsoleLogger
```go
logger := logging.NewConsoleLogger(logging.LevelInfo)
```
- 人类可读的控制台输出
- 支持颜色编码（可选）
- 适合开发和调试

#### SlogLogger
```go
logger := logging.NewSlogLogger(logging.LevelInfo)
```
- 基于 Go 1.21+ 的 `log/slog` 包
- 支持 JSON 和文本格式
- 结构化日志输出
- 高性能

#### MultiLogger
```go
logger := logging.NewMultiLogger(consoleLogger, jsonLogger)
```
- 同时输出到多个目标
- 支持不同格式组合
- 灵活的日志分发

#### NopLogger
```go
logger := logging.NewNopLogger()
```
- 无操作日志器
- 用于性能测试或禁用日志

### 4. 事件发射器

`EventEmitter` 提供便利方法来发射各种事件：

```go
type EventEmitter interface {
    EmitEvent(event Event)
    SetLogger(logger Logger)
    GetLogger() Logger
    SetNodeID(nodeID uint64)
    Close()
}
```

便利方法示例：
```go
emitter.EmitNodeStarting(config)
emitter.EmitStateChanged("follower", "candidate")
emitter.EmitElectionStarted(term)
emitter.EmitLogAppended(count, firstIndex, lastIndex, duration, err)
```

### 5. 依赖注入集成

通过简化的依赖注入容器实现模块化配置：

```go
builder := di.NewApplicationBuilder()
builder.WithConsoleLogging(logging.LevelInfo)
builder.WithRaft(nodeID)

app, err := builder.Build()
emitter, err := app.GetNodeEventEmitter()
```

## 使用示例

### 基本使用

```go
// 创建日志器
logger := logging.NewConsoleLogger(logging.LevelInfo)

// 创建事件发射器
emitter := logging.NewEventEmitter(logger, nodeID)

// 发射事件
emitter.EmitNodeStarting(map[string]interface{}{
    "config": "default",
})
```

### 配置化使用

```go
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
```

### 依赖注入使用

```go
builder := di.NewApplicationBuilder()
builder.WithJSONLogging(logging.LevelDebug, "raft.log")
builder.WithRaft(1)

app, err := builder.Build()
emitter, err := app.GetNodeEventEmitter()
```

## 配置选项

### 日志级别
- `LevelDebug`: 详细调试信息
- `LevelInfo`: 一般信息
- `LevelWarn`: 警告信息
- `LevelError`: 错误信息

### 输出格式
- **控制台格式**: 人类可读，支持颜色
- **JSON 格式**: 结构化，便于解析
- **自定义格式**: 通过实现 `Logger` 接口

### 输出目标
- 标准输出/错误
- 文件输出
- 网络输出（通过自定义实现）

## 性能特性

1. **零分配**: 事件对象可复用，减少 GC 压力
2. **异步处理**: 支持异步日志写入（可选）
3. **级别过滤**: 在发射阶段就过滤不需要的事件
4. **批量处理**: 支持批量写入优化

## 扩展性

### 自定义事件类型

```go
type CustomEvent struct {
    *logging.BaseEvent
    CustomField string
}

func (e *CustomEvent) GetType() logging.EventType {
    return "custom_event"
}
```

### 自定义日志器

```go
type CustomLogger struct {
    level logging.Level
}

func (l *CustomLogger) LogEvent(event logging.Event) {
    // 自定义处理逻辑
}
```

### 自定义发射器

```go
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

## 最佳实践

1. **开发环境**: 使用 `ConsoleLogger` 便于调试
2. **生产环境**: 使用 `SlogLogger` 的 JSON 格式
3. **性能测试**: 使用 `NopLogger` 消除日志开销
4. **监控集成**: 使用 `MultiLogger` 同时输出到文件和监控系统
5. **错误处理**: 始终检查日志器创建和事件发射的错误
6. **资源清理**: 使用 `defer emitter.Close()` 确保资源释放

## 与 Uber fx 的对比

| 特性 | Uber fx | 本实现 |
|------|---------|--------|
| 事件驱动 | ✅ | ✅ |
| 类型安全 | ✅ | ✅ |
| 可扩展性 | ✅ | ✅ |
| 依赖注入 | ✅ | ✅ (简化版) |
| 多日志器 | ✅ | ✅ |
| 配置化 | ✅ | ✅ |
| 性能优化 | ✅ | ✅ |

本实现在保持 fx 核心设计理念的同时，针对 Raft 场景进行了优化和简化。