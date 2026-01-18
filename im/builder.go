package im

import (
	"fmt"
	"os"
	"strconv"

	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/im/internal/core"
)

// Builder IM 服务构建器，支持链式配置
type Builder struct {
	config *core.Config
	err    error
}

// NewBuilder 创建 IM 服务构建器
func NewBuilder() *Builder {
	return &Builder{
		config: &core.Config{
			CacheTTL:          30,
			HeartbeatInterval: 15,
		},
	}
}

// WithServerID 设置服务器 ID
func (b *Builder) WithServerID(serverID string) *Builder {
	if b.err != nil {
		return b
	}
	b.config.ServerID = serverID
	return b
}

// WithGRPCAddr 设置 gRPC 地址
func (b *Builder) WithGRPCAddr(addr string) *Builder {
	if b.err != nil {
		return b
	}
	b.config.GRPCAddr = addr
	return b
}

// WithDB 设置数据库连接
func (b *Builder) WithDB(db *gorm.DB) *Builder {
	if b.err != nil {
		return b
	}
	b.config.DB = db
	return b
}

// WithAuthFunc 设置认证函数
func (b *Builder) WithAuthFunc(authFunc func(token string) (int64, error)) *Builder {
	if b.err != nil {
		return b
	}
	b.config.AuthFunc = authFunc
	return b
}

// WithCacheTTL 设置路由缓存 TTL（秒）
func (b *Builder) WithCacheTTL(seconds int) *Builder {
	if b.err != nil {
		return b
	}
	b.config.CacheTTL = seconds
	return b
}

// WithHeartbeatInterval 设置心跳间隔（秒）
func (b *Builder) WithHeartbeatInterval(seconds int) *Builder {
	if b.err != nil {
		return b
	}
	b.config.HeartbeatInterval = seconds
	return b
}

// FromEnv 从环境变量加载配置
// 支持的环境变量：
//   IM_SERVER_ID      - 服务器 ID
//   IM_GRPC_ADDR      - gRPC 地址
//   IM_CACHE_TTL      - 缓存 TTL（秒）
//   IM_HEARTBEAT      - 心跳间隔（秒）
func (b *Builder) FromEnv() *Builder {
	if b.err != nil {
		return b
	}

	if serverID := os.Getenv("IM_SERVER_ID"); serverID != "" {
		b.config.ServerID = serverID
	}

	if grpcAddr := os.Getenv("IM_GRPC_ADDR"); grpcAddr != "" {
		b.config.GRPCAddr = grpcAddr
	}

	if cacheTTL := os.Getenv("IM_CACHE_TTL"); cacheTTL != "" {
		if ttl, err := strconv.Atoi(cacheTTL); err == nil {
			b.config.CacheTTL = ttl
		}
	}

	if heartbeat := os.Getenv("IM_HEARTBEAT"); heartbeat != "" {
		if interval, err := strconv.Atoi(heartbeat); err == nil {
			b.config.HeartbeatInterval = interval
		}
	}

	return b
}

// Build 构建 IM 服务实例
func (b *Builder) Build() (IMService, error) {
	if b.err != nil {
		return nil, b.err
	}

	// 验证必需参数
	if b.config.ServerID == "" {
		return nil, fmt.Errorf("server_id is required")
	}

	if b.config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	if b.config.AuthFunc == nil {
		return nil, fmt.Errorf("auth function is required")
	}

	return core.NewIMServer(b.config)
}

// MustBuild 构建 IM 服务实例，出错时 panic
func (b *Builder) MustBuild() IMService {
	service, err := b.Build()
	if err != nil {
		panic(err)
	}
	return service
}
