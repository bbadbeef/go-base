package log

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogrusAdapter logrus 适配器，实现 Logger 接口
type LogrusAdapter struct {
	logger *logrus.Logger
}

// NewLogrusAdapter 创建 logrus 适配器
func NewLogrusAdapter(logger *logrus.Logger) *LogrusAdapter {
	return &LogrusAdapter{logger: logger}
}

// Debug 调试日志
func (l *LogrusAdapter) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

// Debugf 调试日志（格式化）
func (l *LogrusAdapter) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

// Info 信息日志
func (l *LogrusAdapter) Info(args ...interface{}) {
	l.logger.Info(args...)
}

// Infof 信息日志（格式化）
func (l *LogrusAdapter) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

// Warn 警告日志
func (l *LogrusAdapter) Warn(args ...interface{}) {
	l.logger.Warn(args...)
}

// Warnf 警告日志（格式化）
func (l *LogrusAdapter) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}

// Error 错误日志
func (l *LogrusAdapter) Error(args ...interface{}) {
	l.logger.Error(args...)
}

// Errorf 错误日志（格式化）
func (l *LogrusAdapter) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

// Fatal 致命错误日志
func (l *LogrusAdapter) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

// Fatalf 致命错误日志（格式化）
func (l *LogrusAdapter) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

// WithField 添加单个字段
func (l *LogrusAdapter) WithField(key string, value interface{}) Logger {
	return &LogrusEntryAdapter{entry: l.logger.WithField(key, value)}
}

// WithFields 添加多个字段
func (l *LogrusAdapter) WithFields(fields map[string]interface{}) Logger {
	return &LogrusEntryAdapter{entry: l.logger.WithFields(fields)}
}

// LogrusEntryAdapter logrus.Entry 适配器
type LogrusEntryAdapter struct {
	entry *logrus.Entry
}

func (l *LogrusEntryAdapter) Debug(args ...interface{}) {
	l.entry.Debug(args...)
}

func (l *LogrusEntryAdapter) Debugf(format string, args ...interface{}) {
	l.entry.Debugf(format, args...)
}

func (l *LogrusEntryAdapter) Info(args ...interface{}) {
	l.entry.Info(args...)
}

func (l *LogrusEntryAdapter) Infof(format string, args ...interface{}) {
	l.entry.Infof(format, args...)
}

func (l *LogrusEntryAdapter) Warn(args ...interface{}) {
	l.entry.Warn(args...)
}

func (l *LogrusEntryAdapter) Warnf(format string, args ...interface{}) {
	l.entry.Warnf(format, args...)
}

func (l *LogrusEntryAdapter) Error(args ...interface{}) {
	l.entry.Error(args...)
}

func (l *LogrusEntryAdapter) Errorf(format string, args ...interface{}) {
	l.entry.Errorf(format, args...)
}

func (l *LogrusEntryAdapter) Fatal(args ...interface{}) {
	l.entry.Fatal(args...)
}

func (l *LogrusEntryAdapter) Fatalf(format string, args ...interface{}) {
	l.entry.Fatalf(format, args...)
}

func (l *LogrusEntryAdapter) WithField(key string, value interface{}) Logger {
	return &LogrusEntryAdapter{entry: l.entry.WithField(key, value)}
}

func (l *LogrusEntryAdapter) WithFields(fields map[string]interface{}) Logger {
	return &LogrusEntryAdapter{entry: l.entry.WithFields(fields)}
}

// LogConfig 日志配置
type LogConfig struct {
	// Level 日志级别: debug, info, warn, error
	Level string
	// LogFile 日志文件路径，为空则输出到 stdout
	LogFile string
	// MaxSize 单个日志文件最大大小(MB)
	MaxSize int
	// MaxBackups 保留的旧日志文件最大数量
	MaxBackups int
	// MaxAge 保留的旧日志文件最大天数
	MaxAge int
	// Compress 是否压缩旧日志文件
	Compress bool
}

// DefaultLogConfig 默认日志配置
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:      "info",
		LogFile:    "",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}
}

// NewLogrusLogger 创建一个配置好的 logrus logger
func NewLogrusLogger(config *LogConfig) *logrus.Logger {
	logger := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 设置输出
	if config.LogFile != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Errorf("Failed to create log directory: %v", err)
			logger.SetOutput(os.Stdout)
			return logger
		}

		// 使用 lumberjack 实现日志滚动
		logger.SetOutput(&lumberjack.Logger{
			Filename:   config.LogFile,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
			LocalTime:  true,
		})
	} else {
		logger.SetOutput(os.Stdout)
	}

	return logger
}

// SetOutput 设置 logrus 输出
func (l *LogrusAdapter) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

// SetLevel 设置 logrus 日志级别
func (l *LogrusAdapter) SetLevel(level string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lvl = logrus.InfoLevel
	}
	l.logger.SetLevel(lvl)
}
