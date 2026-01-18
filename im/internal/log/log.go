package log

import (
	"sync"
)

var (
	defaultLogger Logger
	mu            sync.RWMutex
	once          sync.Once
)

// init 初始化默认 logger
func init() {
	// 使用默认配置创建 logrus logger
	defaultLogger = NewLogrusAdapter(NewLogrusLogger(DefaultLogConfig()))
}

// SetLogger 设置自定义 logger
// logger 可以是任何实现了 Logger 接口的实例
func SetLogger(logger Logger) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger = logger
}

// GetLogger 获取当前 logger
func GetLogger() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger
}

// InitWithLogrus 使用 logrus 配置初始化日志
func InitWithLogrus(config *LogConfig) {
	logrusLogger := NewLogrusLogger(config)
	SetLogger(NewLogrusAdapter(logrusLogger))
}

// SetLogLevel 设置日志级别（仅对 LogrusAdapter 有效）
func SetLogLevel(level string) {
	mu.RLock()
	logger := defaultLogger
	mu.RUnlock()

	if adapter, ok := logger.(*LogrusAdapter); ok {
		adapter.SetLevel(level)
	}
}

// Debug 调试日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 调试日志（格式化）
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info 信息日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 信息日志（格式化）
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn 警告日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 警告日志（格式化）
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error 错误日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 错误日志（格式化）
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal 致命错误日志
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf 致命错误日志（格式化）
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// WithField 添加单个字段（如果 logger 支持）
func WithField(key string, value interface{}) Logger {
	logger := GetLogger()
	if wf, ok := logger.(WithFielder); ok {
		return wf.WithField(key, value)
	}
	return logger
}

// WithFields 添加多个字段（如果 logger 支持）
func WithFields(fields map[string]interface{}) Logger {
	logger := GetLogger()
	if wf, ok := logger.(WithFielder); ok {
		return wf.WithFields(fields)
	}
	return logger
}
