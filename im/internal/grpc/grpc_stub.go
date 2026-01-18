package imgrpc

import (
	"context"

	"github.com/bbadbeef/go-base/im/internal/model"

	"google.golang.org/grpc"
)

// IMServerClient gRPC 客户端接口（临时桩代码）
type IMServerClient interface {
	ForwardMessage(ctx context.Context, in *ForwardMessageRequest, opts ...grpc.CallOption) (*ForwardMessageResponse, error)
}

// IMServerServer gRPC 服务端接口（临时桩代码）
type IMServerServer interface {
	ForwardMessage(context.Context, *ForwardMessageRequest) (*ForwardMessageResponse, error)
}

// ForwardMessageRequest 转发消息请求
type ForwardMessageRequest struct {
	ToUserID   int64  `json:"to_user_id"`
	MsgID      string `json:"msg_id"`
	FromUserID int64  `json:"from_user_id"`
	Content    string `json:"content"`
	MsgType    int32  `json:"msg_type"`
	ClientTime int64  `json:"client_time"`
	ServerTime int64  `json:"server_time"`
}

// ForwardMessageResponse 转发消息响应
type ForwardMessageResponse struct {
	Delivered bool   `json:"delivered"`
	Error     string `json:"error"`
}

// RegisterIMServerServer 注册 gRPC 服务（临时桩代码）
func RegisterIMServerServer(s *grpc.Server, srv IMServerServer) {
	// TODO: 使用 protobuf 生成的代码替换
}

// NewIMServerClient 创建 gRPC 客户端（临时桩代码）
func NewIMServerClient(cc *grpc.ClientConn) IMServerClient {
	// TODO: 使用 protobuf 生成的代码替换
	return nil
}

// 辅助函数：将 model.Message 转换为 ForwardMessageRequest
func MessageToForwardRequest(msg *model.Message) *ForwardMessageRequest {
	return &ForwardMessageRequest{
		ToUserID:   msg.ToUserID,
		MsgID:      msg.MsgID,
		FromUserID: msg.FromUserID,
		Content:    msg.Content,
		MsgType:    int32(msg.MsgType),
		ClientTime: msg.ClientTime,
		ServerTime: msg.ServerTime,
	}
}
