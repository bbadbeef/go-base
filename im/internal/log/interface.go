package log

// Logger 日志接口
// 任何实现了这个接口的日志实例都可以传入 IM 模块使用
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

// WithFielder 支持结构化日志的接口（可选）
type WithFielder interface {
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
}
