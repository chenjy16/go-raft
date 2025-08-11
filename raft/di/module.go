package di

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/chenjianyu/go-raft/raft/logging"
)

// Container 简化的依赖注入容器
type Container struct {
	services map[string]interface{}
	mu       sync.RWMutex
}

// NewContainer 创建新的容器
func NewContainer() *Container {
	return &Container{
		services: make(map[string]interface{}),
	}
}

// Register 注册服务
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// Get 获取服务
func (c *Container) Get(name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service, exists := c.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}
	return service, nil
}

// GetTyped 获取指定类型的服务
func (c *Container) GetTyped(name string, target interface{}) error {
	service, err := c.Get(name)
	if err != nil {
		return err
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	serviceValue := reflect.ValueOf(service)
	targetType := targetValue.Elem().Type()

	if !serviceValue.Type().AssignableTo(targetType) {
		return fmt.Errorf("service type %v is not assignable to target type %v",
			serviceValue.Type(), targetType)
	}

	targetValue.Elem().Set(serviceValue)
	return nil
}

// LoggingModule 日志模块
type LoggingModule struct {
	config logging.LoggerConfig
}

// NewLoggingModule 创建日志模块
func NewLoggingModule(config logging.LoggerConfig) *LoggingModule {
	return &LoggingModule{
		config: config,
	}
}

// Configure 配置依赖注入容器
func (m *LoggingModule) Configure(container *Container) error {
	// 注册日志器工厂
	factory := logging.NewLoggerFactory()
	container.Register("loggerFactory", factory)

	// 注册日志器
	logger, err := factory.CreateLogger(m.config)
	if err != nil {
		return err
	}
	container.Register("logger", logger)

	// 注册事件发射器工厂
	emitterFactory := &DefaultEventEmitterFactory{}
	container.Register("eventEmitterFactory", emitterFactory)

	// 注册默认事件发射器
	emitter := emitterFactory.CreateEventEmitter(logger, 0) // nodeID 将在运行时设置
	container.Register("eventEmitter", emitter)

	return nil
}

// EventEmitterFactory 事件发射器工厂接口
type EventEmitterFactory interface {
	CreateEventEmitter(logger logging.Logger, nodeID uint64) logging.EventEmitter
}

// DefaultEventEmitterFactory 默认事件发射器工厂
type DefaultEventEmitterFactory struct{}

func (f *DefaultEventEmitterFactory) CreateEventEmitter(logger logging.Logger, nodeID uint64) logging.EventEmitter {
	return logging.NewEventEmitter(logger, nodeID)
}

// RaftModule Raft 核心模块
type RaftModule struct {
	nodeID uint64
}

// NewRaftModule 创建 Raft 模块
func NewRaftModule(nodeID uint64) *RaftModule {
	return &RaftModule{
		nodeID: nodeID,
	}
}

// Configure 配置依赖注入容器
func (m *RaftModule) Configure(container *Container) error {
	// 注册节点 ID
	container.Register("nodeID", m.nodeID)

	// 获取日志器和工厂
	logger, err := container.Get("logger")
	if err != nil {
		return err
	}

	factory, err := container.Get("eventEmitterFactory")
	if err != nil {
		return err
	}

	// 注册带节点 ID 的事件发射器
	emitterFactory := factory.(EventEmitterFactory)
	nodeEmitter := emitterFactory.CreateEventEmitter(logger.(logging.Logger), m.nodeID)
	container.Register("nodeEventEmitter", nodeEmitter)

	return nil
}

// ApplicationBuilder 应用构建器
type ApplicationBuilder struct {
	container *Container
	modules   []Module
}

// Module 模块接口
type Module interface {
	Configure(container *Container) error
}

// NewApplicationBuilder 创建应用构建器
func NewApplicationBuilder() *ApplicationBuilder {
	return &ApplicationBuilder{
		container: NewContainer(),
		modules:   make([]Module, 0),
	}
}

// AddModule 添加模块
func (b *ApplicationBuilder) AddModule(module Module) *ApplicationBuilder {
	b.modules = append(b.modules, module)
	return b
}

// WithLogging 添加日志模块
func (b *ApplicationBuilder) WithLogging(config logging.LoggerConfig) *ApplicationBuilder {
	return b.AddModule(NewLoggingModule(config))
}

// WithDefaultLogging 添加默认日志模块
func (b *ApplicationBuilder) WithDefaultLogging() *ApplicationBuilder {
	return b.WithLogging(logging.DefaultLoggerConfig())
}

// WithConsoleLogging 添加控制台日志模块
func (b *ApplicationBuilder) WithConsoleLogging(level logging.Level) *ApplicationBuilder {
	config := logging.NewLoggerBuilder().
		WithType(logging.LoggerTypeConsole).
		WithLevel(level).
		WithOutput("stderr").
		GetConfig()
	return b.WithLogging(config)
}

// WithJSONLogging 添加 JSON 格式日志模块
func (b *ApplicationBuilder) WithJSONLogging(level logging.Level, output string) *ApplicationBuilder {
	config := logging.NewLoggerBuilder().
		WithType(logging.LoggerTypeSlog).
		WithLevel(level).
		WithOutput(output).
		WithJSONFormat(true).
		GetConfig()
	return b.WithLogging(config)
}

// WithRaft 添加 Raft 模块
func (b *ApplicationBuilder) WithRaft(nodeID uint64) *ApplicationBuilder {
	return b.AddModule(NewRaftModule(nodeID))
}

// Build 构建应用
func (b *ApplicationBuilder) Build() (*Application, error) {
	// 配置所有模块
	for _, module := range b.modules {
		if err := module.Configure(b.container); err != nil {
			return nil, err
		}
	}

	return &Application{
		container: b.container,
	}, nil
}

// Application 应用
type Application struct {
	container *Container
}

// GetContainer 获取容器
func (a *Application) GetContainer() *Container {
	return a.container
}

// GetLogger 获取日志器
func (a *Application) GetLogger() (logging.Logger, error) {
	var logger logging.Logger
	err := a.container.GetTyped("logger", &logger)
	return logger, err
}

// GetEventEmitter 获取事件发射器
func (a *Application) GetEventEmitter() (logging.EventEmitter, error) {
	var emitter logging.EventEmitter
	err := a.container.GetTyped("eventEmitter", &emitter)
	return emitter, err
}

// GetNodeEventEmitter 获取节点事件发射器
func (a *Application) GetNodeEventEmitter() (logging.EventEmitter, error) {
	var emitter logging.EventEmitter
	err := a.container.GetTyped("nodeEventEmitter", &emitter)
	return emitter, err
}

// Close 关闭应用
func (a *Application) Close() error {
	// 关闭日志器
	if logger, err := a.GetLogger(); err == nil {
		if err := logger.Close(); err != nil {
			return err
		}
	}

	// 关闭事件发射器
	if emitter, err := a.GetEventEmitter(); err == nil {
		if err := emitter.Close(); err != nil {
			return err
		}
	}

	return nil
}
