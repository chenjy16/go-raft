package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger 日志器接口 - 核心抽象
type Logger interface {
	// LogEvent 记录事件
	LogEvent(event Event)
	// SetLevel 设置日志级别
	SetLevel(level Level)
	// GetLevel 获取日志级别
	GetLevel() Level
	// WithFields 添加字段
	WithFields(fields map[string]interface{}) Logger
	// Close 关闭日志器
	Close() error
}

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelOff
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelOff:
		return "OFF"
	default:
		return "UNKNOWN"
	}
}

// ConsoleLogger 控制台日志器实现
type ConsoleLogger struct {
	level  Level
	writer io.Writer
	fields map[string]interface{}
}

// NewConsoleLogger 创建控制台日志器
func NewConsoleLogger(level Level) *ConsoleLogger {
	return &ConsoleLogger{
		level:  level,
		writer: os.Stderr,
		fields: make(map[string]interface{}),
	}
}

// NewConsoleLoggerWithWriter 创建带自定义输出的控制台日志器
func NewConsoleLoggerWithWriter(level Level, writer io.Writer) *ConsoleLogger {
	return &ConsoleLogger{
		level:  level,
		writer: writer,
		fields: make(map[string]interface{}),
	}
}

func (l *ConsoleLogger) LogEvent(event Event) {
	if l.shouldLog(event) {
		l.writeEvent(event)
	}
}

func (l *ConsoleLogger) SetLevel(level Level) {
	l.level = level
}

func (l *ConsoleLogger) GetLevel() Level {
	return l.level
}

func (l *ConsoleLogger) WithFields(fields map[string]interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	
	return &ConsoleLogger{
		level:  l.level,
		writer: l.writer,
		fields: newFields,
	}
}

func (l *ConsoleLogger) Close() error {
	return nil
}

func (l *ConsoleLogger) shouldLog(event Event) bool {
	if l.level == LevelOff {
		return false
	}
	
	eventLevel := l.getEventLevel(event)
	return eventLevel >= l.level
}

func (l *ConsoleLogger) getEventLevel(event Event) Level {
	switch event.GetEventType() {
	case EventError:
		return LevelError
	case EventWarning:
		return LevelWarn
	case EventNodeStarting, EventNodeStarted, EventNodeStopping, EventNodeStopped,
		 EventStateChanged, EventTermChanged, EventLeaderChanged,
		 EventElectionStarted, EventElectionWon, EventElectionLost:
		return LevelInfo
	default:
		return LevelDebug
	}
}

func (l *ConsoleLogger) writeEvent(event Event) {
	timestamp := event.GetTimestamp().Format("2006-01-02 15:04:05.000")
	level := l.getEventLevel(event).String()
	eventType := string(event.GetEventType())
	nodeID := event.GetNodeID()
	
	// 构建基础信息
	parts := []string{
		fmt.Sprintf("[%s]", timestamp),
		fmt.Sprintf("[%s]", level),
		fmt.Sprintf("[Node-%d]", nodeID),
		fmt.Sprintf("[%s]", eventType),
	}
	
	// 添加事件特定信息
	eventInfo := l.formatEventInfo(event)
	if eventInfo != "" {
		parts = append(parts, eventInfo)
	}
	
	// 添加额外字段
	if len(l.fields) > 0 {
		fieldsStr := l.formatFields(l.fields)
		parts = append(parts, fieldsStr)
	}
	
	message := strings.Join(parts, " ")
	fmt.Fprintln(l.writer, message)
}

func (l *ConsoleLogger) formatEventInfo(event Event) string {
	switch e := event.(type) {
	case *NodeStartedEvent:
		if e.Error != nil {
			return fmt.Sprintf("duration=%v error=%v", e.Duration, e.Error)
		}
		return fmt.Sprintf("duration=%v", e.Duration)
	case *StateChangedEvent:
		return fmt.Sprintf("old_state=%s new_state=%s", e.OldState, e.NewState)
	case *TermChangedEvent:
		return fmt.Sprintf("old_term=%d new_term=%d", e.OldTerm, e.NewTerm)
	case *LeaderChangedEvent:
		return fmt.Sprintf("old_leader=%d new_leader=%d", e.OldLeader, e.NewLeader)
	case *LogAppendedEvent:
		if e.Error != nil {
			return fmt.Sprintf("entries=%d first_index=%d last_index=%d duration=%v error=%v", 
				e.EntryCount, e.FirstIndex, e.LastIndex, e.Duration, e.Error)
		}
		return fmt.Sprintf("entries=%d first_index=%d last_index=%d duration=%v", 
			e.EntryCount, e.FirstIndex, e.LastIndex, e.Duration)
	case *MessageSentEvent:
		if e.Error != nil {
			return fmt.Sprintf("type=%s to=%d term=%d duration=%v success=%t error=%v", 
				e.MessageType, e.To, e.Term, e.Duration, e.Success, e.Error)
		}
		return fmt.Sprintf("type=%s to=%d term=%d duration=%v success=%t", 
			e.MessageType, e.To, e.Term, e.Duration, e.Success)
	case *ElectionWonEvent:
		return fmt.Sprintf("term=%d votes=%d duration=%v", e.Term, e.VoteCount, e.Duration)
	case *ErrorEvent:
		return fmt.Sprintf("context=%s error=%v", e.Context, e.Error)
	case *WarningEvent:
		return fmt.Sprintf("context=%s message=%s", e.Context, e.Message)
	default:
		return ""
	}
}

func (l *ConsoleLogger) formatFields(fields map[string]interface{}) string {
	var parts []string
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, " ")
}

// SlogLogger 基于 slog 的日志器实现
type SlogLogger struct {
	logger *slog.Logger
	level  Level
	fields map[string]interface{}
}

// NewSlogLogger 创建 slog 日志器
func NewSlogLogger(level Level) *SlogLogger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stderr, opts)
	logger := slog.New(handler)
	
	return &SlogLogger{
		logger: logger,
		level:  level,
		fields: make(map[string]interface{}),
	}
}

// NewSlogLoggerWithHandler 创建带自定义处理器的 slog 日志器
func NewSlogLoggerWithHandler(level Level, handler slog.Handler) *SlogLogger {
	return &SlogLogger{
		logger: slog.New(handler),
		level:  level,
		fields: make(map[string]interface{}),
	}
}

func (l *SlogLogger) LogEvent(event Event) {
	if l.shouldLog(event) {
		l.writeEvent(event)
	}
}

func (l *SlogLogger) SetLevel(level Level) {
	l.level = level
}

func (l *SlogLogger) GetLevel() Level {
	return l.level
}

func (l *SlogLogger) WithFields(fields map[string]interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	
	return &SlogLogger{
		logger: l.logger,
		level:  l.level,
		fields: newFields,
	}
}

func (l *SlogLogger) Close() error {
	return nil
}

func (l *SlogLogger) shouldLog(event Event) bool {
	if l.level == LevelOff {
		return false
	}
	
	eventLevel := l.getEventLevel(event)
	return eventLevel >= l.level
}

func (l *SlogLogger) getEventLevel(event Event) Level {
	switch event.GetEventType() {
	case EventError:
		return LevelError
	case EventWarning:
		return LevelWarn
	case EventNodeStarting, EventNodeStarted, EventNodeStopping, EventNodeStopped,
		 EventStateChanged, EventTermChanged, EventLeaderChanged,
		 EventElectionStarted, EventElectionWon, EventElectionLost:
		return LevelInfo
	default:
		return LevelDebug
	}
}

func (l *SlogLogger) writeEvent(event Event) {
	slogLevel := l.toSlogLevel(l.getEventLevel(event))
	
	// 构建属性
	attrs := []slog.Attr{
		slog.String("event_type", string(event.GetEventType())),
		slog.Uint64("node_id", event.GetNodeID()),
		slog.Time("timestamp", event.GetTimestamp()),
	}
	
	// 添加事件特定属性
	eventAttrs := l.getEventAttrs(event)
	attrs = append(attrs, eventAttrs...)
	
	// 添加额外字段
	for k, v := range l.fields {
		attrs = append(attrs, slog.Any(k, v))
	}
	
	l.logger.LogAttrs(nil, slogLevel, "raft_event", attrs...)
}

func (l *SlogLogger) toSlogLevel(level Level) slog.Level {
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

func (l *SlogLogger) getEventAttrs(event Event) []slog.Attr {
	switch e := event.(type) {
	case *NodeStartedEvent:
		attrs := []slog.Attr{slog.Duration("duration", e.Duration)}
		if e.Error != nil {
			attrs = append(attrs, slog.String("error", e.Error.Error()))
		}
		return attrs
	case *StateChangedEvent:
		return []slog.Attr{
			slog.String("old_state", e.OldState),
			slog.String("new_state", e.NewState),
		}
	case *TermChangedEvent:
		return []slog.Attr{
			slog.Uint64("old_term", e.OldTerm),
			slog.Uint64("new_term", e.NewTerm),
		}
	case *LeaderChangedEvent:
		return []slog.Attr{
			slog.Uint64("old_leader", e.OldLeader),
			slog.Uint64("new_leader", e.NewLeader),
		}
	case *LogAppendedEvent:
		attrs := []slog.Attr{
			slog.Int("entry_count", e.EntryCount),
			slog.Uint64("first_index", e.FirstIndex),
			slog.Uint64("last_index", e.LastIndex),
			slog.Duration("duration", e.Duration),
		}
		if e.Error != nil {
			attrs = append(attrs, slog.String("error", e.Error.Error()))
		}
		return attrs
	case *MessageSentEvent:
		attrs := []slog.Attr{
			slog.String("message_type", e.MessageType),
			slog.Uint64("to", e.To),
			slog.Uint64("term", e.Term),
			slog.Duration("duration", e.Duration),
			slog.Bool("success", e.Success),
		}
		if e.Error != nil {
			attrs = append(attrs, slog.String("error", e.Error.Error()))
		}
		return attrs
	case *ElectionWonEvent:
		return []slog.Attr{
			slog.Uint64("term", e.Term),
			slog.Int("vote_count", e.VoteCount),
			slog.Duration("duration", e.Duration),
		}
	case *ErrorEvent:
		attrs := []slog.Attr{
			slog.String("context", e.Context),
			slog.String("error", e.Error.Error()),
		}
		if e.Details != nil {
			detailsJson, _ := json.Marshal(e.Details)
			attrs = append(attrs, slog.String("details", string(detailsJson)))
		}
		return attrs
	case *WarningEvent:
		attrs := []slog.Attr{
			slog.String("context", e.Context),
			slog.String("message", e.Message),
		}
		if e.Details != nil {
			detailsJson, _ := json.Marshal(e.Details)
			attrs = append(attrs, slog.String("details", string(detailsJson)))
		}
		return attrs
	default:
		return nil
	}
}

// NopLogger 空日志器实现
type NopLogger struct{}

// NewNopLogger 创建空日志器
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

func (l *NopLogger) LogEvent(event Event) {}

func (l *NopLogger) SetLevel(level Level) {}

func (l *NopLogger) GetLevel() Level {
	return LevelOff
}

func (l *NopLogger) WithFields(fields map[string]interface{}) Logger {
	return l
}

func (l *NopLogger) Close() error {
	return nil
}

// MultiLogger 多日志器组合实现
type MultiLogger struct {
	loggers []Logger
	level   Level
}

// NewMultiLogger 创建多日志器
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{
		loggers: loggers,
		level:   LevelInfo,
	}
}

func (l *MultiLogger) LogEvent(event Event) {
	for _, logger := range l.loggers {
		logger.LogEvent(event)
	}
}

func (l *MultiLogger) SetLevel(level Level) {
	l.level = level
	for _, logger := range l.loggers {
		logger.SetLevel(level)
	}
}

func (l *MultiLogger) GetLevel() Level {
	return l.level
}

func (l *MultiLogger) WithFields(fields map[string]interface{}) Logger {
	newLoggers := make([]Logger, len(l.loggers))
	for i, logger := range l.loggers {
		newLoggers[i] = logger.WithFields(fields)
	}
	return &MultiLogger{
		loggers: newLoggers,
		level:   l.level,
	}
}

func (l *MultiLogger) Close() error {
	for _, logger := range l.loggers {
		if err := logger.Close(); err != nil {
			return err
		}
	}
	return nil
}