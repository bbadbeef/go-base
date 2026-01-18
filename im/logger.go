package im

import (
	"github.com/bbadbeef/go-base/im/internal/log"
)

// Logger 日志接口
// 任何实现了这个接口的日志实例都可以传入 IM 模块使用
type Logger = log.Logger

// LogConfig 日志配置（用于 logrus）
type LogConfig = log.LogConfig

// DefaultLogConfig 默认日志配置
func DefaultLogConfig() *LogConfig {
	return log.DefaultLogConfig()
}

// InitLogger 初始化日志（使用 logrus）
func InitLogger(config *LogConfig) {
	log.InitWithLogrus(config)
}

// SetLogger 设置自定义 logger
// logger 可以是任何实现了 Logger 接口的实例，不限于 logrus
// 例如：zap、zerolog 或你自己实现的 logger
func SetLogger(logger Logger) {
	log.SetLogger(logger)
}

// GetLogger 获取当前的 logger
func GetLogger() Logger {
	return log.GetLogger()
}

// SetLogLevel 设置日志级别: debug, info, warn, error
// 注意：此方法仅对内置的 logrus adapter 有效
func SetLogLevel(level string) {
	log.SetLogLevel(level)
}

// Debug 调试日志
func Debug(args ...interface{}) {
	log.Debug(args...)
}

// Debugf 调试日志（格式化）
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// Info 信息日志
func Info(args ...interface{}) {
	log.Info(args...)
}

// Infof 信息日志（格式化）
func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Warn 警告日志
func Warn(args ...interface{}) {
	log.Warn(args...)
}

// Warnf 警告日志（格式化）
func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

// Error 错误日志
func Error(args ...interface{}) {
	log.Error(args...)
}

// Errorf 错误日志（格式化）
func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

// Fatal 致命错误日志
func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

// Fatalf 致命错误日志（格式化）
func Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

// WithField 添加单个字段（如果 logger 支持）
func WithField(key string, value interface{}) Logger {
	return log.WithField(key, value)
}

// WithFields 添加多个字段（如果 logger 支持）
func WithFields(fields map[string]interface{}) Logger {
	return log.WithFields(fields)
}
