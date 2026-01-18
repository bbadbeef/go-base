package core

import "gorm.io/gorm"

// Config IM 模块配置
type Config struct {
	// ServerID 当前节点唯一标识
	ServerID string

	// GRPCAddr gRPC 监听地址，用于节点间通信 (例如: "0.0.0.0:50051")
	GRPCAddr string

	// DB 数据库连接（由主应用提供）
	DB *gorm.DB

	// AuthFunc 认证函数，验证 Token 并返回用户 ID
	// 由主应用实现，用于验证 WebSocket 连接时的 Token
	AuthFunc func(token string) (userID int64, err error)

	// CacheTTL 路由缓存时间（秒），默认 30 秒
	CacheTTL int

	// HeartbeatInterval 心跳间隔（秒），默认 15 秒
	HeartbeatInterval int
}
