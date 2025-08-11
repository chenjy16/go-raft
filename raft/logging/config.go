package logging

import (
	"io"
	"log/slog"
	"os"
)

// LoggerType 日志器类型
type LoggerType string

const (
	LoggerTypeConsole LoggerType = "console"
	LoggerTypeSlog    LoggerType = "slog"
	LoggerTypeNop     LoggerType = "nop"
	LoggerTypeMulti   LoggerType = "multi"
)

// LoggerConfig 日志器配置
type LoggerConfig struct {
	// Type 日志器类型
	Type LoggerType `json:"type" yaml:"type"`
	// Level 日志级别
	Level Level `json:"level" yaml:"level"`
	// Output 输出配置
	Output OutputConfig `json:"output" yaml:"output"`
	// Format 格式配置
	Format FormatConfig `json:"format" yaml:"format"`
	// Fields 额外字段
	Fields map[string]interface{} `json:"fields" yaml:"fields"`
	// Children 子日志器配置（用于 MultiLogger）
	Children []LoggerConfig `json:"children" yaml:"children"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	// Writer 输出目标 ("stdout", "stderr", 或文件路径)
	Writer string `json:"writer" yaml:"writer"`
	// File 文件配置
	File FileConfig `json:"file" yaml:"file"`
}

// FileConfig 文件配置
type FileConfig struct {
	// Path 文件路径
	Path string `json:"path" yaml:"path"`
	// MaxSize 最大文件大小 (MB)
	MaxSize int `json:"max_size" yaml:"max_size"`
	// MaxBackups 最大备份数量
	MaxBackups int `json:"max_backups" yaml:"max_backups"`
	// MaxAge 最大保留天数
	MaxAge int `json:"max_age" yaml:"max_age"`
	// Compress 是否压缩
	Compress bool `json:"compress" yaml:"compress"`
}

// FormatConfig 格式配置
type FormatConfig struct {
	// JSON 是否使用 JSON 格式
	JSON bool `json:"json" yaml:"json"`
	// TimeFormat 时间格式
	TimeFormat string `json:"time_format" yaml:"time_format"`
	// AddSource 是否添加源码信息
	AddSource bool `json:"add_source" yaml:"add_source"`
}

// DefaultLoggerConfig 默认日志器配置
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Type:  LoggerTypeConsole,
		Level: LevelInfo,
		Output: OutputConfig{
			Writer: "stderr",
		},
		Format: FormatConfig{
			JSON:       false,
			TimeFormat: "2006-01-02 15:04:05.000",
			AddSource:  false,
		},
		Fields: make(map[string]interface{}),
	}
}

// LoggerFactory 日志器工厂
type LoggerFactory struct{}

// NewLoggerFactory 创建日志器工厂
func NewLoggerFactory() *LoggerFactory {
	return &LoggerFactory{}
}

// CreateLogger 根据配置创建日志器
func (f *LoggerFactory) CreateLogger(config LoggerConfig) (Logger, error) {
	switch config.Type {
	case LoggerTypeConsole:
		return f.createConsoleLogger(config)
	case LoggerTypeSlog:
		return f.createSlogLogger(config)
	case LoggerTypeNop:
		return f.createNopLogger(config)
	case LoggerTypeMulti:
		return f.createMultiLogger(config)
	default:
		return f.createConsoleLogger(config)
	}
}

func (f *LoggerFactory) createConsoleLogger(config LoggerConfig) (Logger, error) {
	writer, err := f.createWriter(config.Output)
	if err != nil {
		return nil, err
	}
	
	logger := NewConsoleLoggerWithWriter(config.Level, writer)
	
	if len(config.Fields) > 0 {
		logger = logger.WithFields(config.Fields).(*ConsoleLogger)
	}
	
	return logger, nil
}

func (f *LoggerFactory) createSlogLogger(config LoggerConfig) (Logger, error) {
	writer, err := f.createWriter(config.Output)
	if err != nil {
		return nil, err
	}
	
	opts := &slog.HandlerOptions{
		Level:     f.toSlogLevel(config.Level),
		AddSource: config.Format.AddSource,
	}
	
	var handler slog.Handler
	if config.Format.JSON {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}
	
	logger := NewSlogLoggerWithHandler(config.Level, handler)
	
	if len(config.Fields) > 0 {
		logger = logger.WithFields(config.Fields).(*SlogLogger)
	}
	
	return logger, nil
}

func (f *LoggerFactory) createNopLogger(config LoggerConfig) (Logger, error) {
	return NewNopLogger(), nil
}

func (f *LoggerFactory) createMultiLogger(config LoggerConfig) (Logger, error) {
	var loggers []Logger
	
	for _, childConfig := range config.Children {
		childLogger, err := f.CreateLogger(childConfig)
		if err != nil {
			return nil, err
		}
		loggers = append(loggers, childLogger)
	}
	
	multiLogger := NewMultiLogger(loggers...)
	multiLogger.SetLevel(config.Level)
	
	if len(config.Fields) > 0 {
		multiLogger = multiLogger.WithFields(config.Fields).(*MultiLogger)
	}
	
	return multiLogger, nil
}

func (f *LoggerFactory) createWriter(config OutputConfig) (io.Writer, error) {
	switch config.Writer {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	case "":
		return os.Stderr, nil
	default:
		// 文件输出
		file, err := os.OpenFile(config.Writer, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}

func (f *LoggerFactory) toSlogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LoggerBuilder 日志器构建器
type LoggerBuilder struct {
	config LoggerConfig
}

// NewLoggerBuilder 创建日志器构建器
func NewLoggerBuilder() *LoggerBuilder {
	return &LoggerBuilder{
		config: DefaultLoggerConfig(),
	}
}

// WithType 设置日志器类型
func (b *LoggerBuilder) WithType(loggerType LoggerType) *LoggerBuilder {
	b.config.Type = loggerType
	return b
}

// WithLevel 设置日志级别
func (b *LoggerBuilder) WithLevel(level Level) *LoggerBuilder {
	b.config.Level = level
	return b
}

// WithOutput 设置输出
func (b *LoggerBuilder) WithOutput(writer string) *LoggerBuilder {
	b.config.Output.Writer = writer
	return b
}

// WithJSONFormat 设置 JSON 格式
func (b *LoggerBuilder) WithJSONFormat(json bool) *LoggerBuilder {
	b.config.Format.JSON = json
	return b
}

// WithTimeFormat 设置时间格式
func (b *LoggerBuilder) WithTimeFormat(format string) *LoggerBuilder {
	b.config.Format.TimeFormat = format
	return b
}

// WithAddSource 设置是否添加源码信息
func (b *LoggerBuilder) WithAddSource(addSource bool) *LoggerBuilder {
	b.config.Format.AddSource = addSource
	return b
}

// WithFields 设置额外字段
func (b *LoggerBuilder) WithFields(fields map[string]interface{}) *LoggerBuilder {
	if b.config.Fields == nil {
		b.config.Fields = make(map[string]interface{})
	}
	for k, v := range fields {
		b.config.Fields[k] = v
	}
	return b
}

// WithField 设置单个字段
func (b *LoggerBuilder) WithField(key string, value interface{}) *LoggerBuilder {
	if b.config.Fields == nil {
		b.config.Fields = make(map[string]interface{})
	}
	b.config.Fields[key] = value
	return b
}

// AddChild 添加子日志器（用于 MultiLogger）
func (b *LoggerBuilder) AddChild(childConfig LoggerConfig) *LoggerBuilder {
	b.config.Children = append(b.config.Children, childConfig)
	return b
}

// Build 构建日志器
func (b *LoggerBuilder) Build() (Logger, error) {
	factory := NewLoggerFactory()
	return factory.CreateLogger(b.config)
}

// GetConfig 获取配置
func (b *LoggerBuilder) GetConfig() LoggerConfig {
	return b.config
}