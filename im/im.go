// Package im 提供嵌入式即时通讯(IM)功能模块
// 支持单聊、群聊、消息状态追踪、分布式节点间消息路由
package im

import (
	"context"
	"errors"
	"net/http"

	"github.com/bbadbeef/go-base/im/internal/core"
	"github.com/bbadbeef/go-base/im/internal/model"
)

// 重新导出类型给外部使用
type (
	Config                = core.Config
	Message               = model.Message
	Session               = model.Session
	SendMessageRequest    = model.SendMessageRequest
	GetMessagesRequest    = model.GetMessagesRequest
	Group                 = model.Group
	GroupMember           = model.GroupMember
)

// 重新导出消息类型常量
const (
	MsgTypeText  = model.MsgTypeText
	MsgTypeImage = model.MsgTypeImage
	MsgTypeVoice = model.MsgTypeVoice
	MsgTypeVideo = model.MsgTypeVideo
	MsgTypeFile  = model.MsgTypeFile
)

// 重新导出消息状态常量
const (
	MsgStatusSending   = model.MsgStatusSending
	MsgStatusSent      = model.MsgStatusSent
	MsgStatusDelivered = model.MsgStatusDelivered
	MsgStatusRead      = model.MsgStatusRead
	MsgStatusFailed    = model.MsgStatusFailed
)

// 重新导出会话类型常量
const (
	SessionTypeSingle = model.SessionTypeSingle
	SessionTypeGroup  = model.SessionTypeGroup
)

// IMService IM 服务接口
type IMService interface {
	// Start 启动 IM 服务
	// ctx: 上下文，用于优雅关闭
	Start(ctx context.Context) error

	// Stop 停止 IM 服务
	Stop() error

	// WebSocketHandler 获取 WebSocket Handler
	// 用于嵌入到主应用的 HTTP 路由中
	// 示例: http.HandleFunc("/ws", imService.WebSocketHandler())
	WebSocketHandler() http.HandlerFunc

	// SendMessage 发送消息（主动推送，如系统消息）
	SendMessage(ctx context.Context, req *SendMessageRequest) error

	// IsUserOnline 检查用户是否在线
	IsUserOnline(userID int64) bool

	// GetSessions 获取用户的会话列表
	GetSessions(ctx context.Context, userID int64) ([]*Session, error)

	// GetMessages 获取历史消息
	GetMessages(ctx context.Context, req *GetMessagesRequest) ([]*Message, error)

	// MarkAsRead 标记消息为已读
	MarkAsRead(ctx context.Context, userID int64, msgIDs []string) error

	// OnMessage 设置消息回调
	// 当收到新消息时触发（主应用可监听此事件做额外处理）
	OnMessage(handler func(*Message))

	// OnUserOnline 设置用户上线回调
	OnUserOnline(handler func(userID int64))

	// OnUserOffline 设置用户下线回调
	OnUserOffline(handler func(userID int64))
}

// New 创建 IM 服务实例
func New(config *Config) (IMService, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}

	if config.ServerID == "" {
		return nil, errors.New("server_id is required")
	}

	if config.DB == nil {
		return nil, errors.New("database connection is required")
	}

	if config.AuthFunc == nil {
		return nil, errors.New("auth function is required")
	}

	// 设置默认值
	if config.CacheTTL == 0 {
		config.CacheTTL = 30
	}

	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 15
	}

	return core.NewIMServer(config)
}
