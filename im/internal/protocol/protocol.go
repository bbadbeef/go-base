package protocol

// WebSocket 消息类型
const (
	WSMsgTypePing             = "ping"              // 心跳请求
	WSMsgTypePong             = "pong"              // 心跳响应
	WSMsgTypeChatMsg          = "chat_msg"          // 发送聊天消息
	WSMsgTypeGroupMsg         = "group_msg"         // 发送群聊消息
	WSMsgTypeAck              = "ack"               // 消息确认
	WSMsgTypeStatusUpdate     = "status_update"     // 消息状态更新
	WSMsgTypeDeliveredReceipt = "delivered_receipt" // 送达回执
	WSMsgTypeReadReceipt      = "read_receipt"      // 已读回执
)

// WSMessage WebSocket 消息包装
type WSMessage struct {
	Type      string      `json:"type"`       // 消息类型
	MsgID     string      `json:"msg_id"`     // 消息 ID
	Data      interface{} `json:"data"`       // 消息数据
	Timestamp int64       `json:"timestamp"`  // 时间戳
}

// WSChatMessage 客户端发送的聊天消息
type WSChatMessage struct {
	MsgID      string `json:"msg_id"`       // 消息 ID（客户端生成 UUID）
	ToUserID   int64  `json:"to_user_id"`   // 接收者用户 ID
	Content    string `json:"content"`      // 消息内容
	MsgType    int    `json:"msg_type"`     // 消息类型
	FileID     string `json:"file_id"`      // 文件ID（多媒体消息）
	ClientTime int64  `json:"client_time"`  // 客户端时间戳
}

// WSGroupMessage 客户端发送的群聊消息
type WSGroupMessage struct {
	MsgID      string `json:"msg_id"`       // 消息 ID
	GroupID    int64  `json:"group_id"`     // 群组 ID
	Content    string `json:"content"`      // 消息内容
	MsgType    int    `json:"msg_type"`     // 消息类型
	FileID     string `json:"file_id"`      // 文件ID（多媒体消息）
	ClientTime int64  `json:"client_time"`  // 客户端时间戳
}

// WSAckMessage 服务端发送的 ACK 确认
type WSAckMessage struct {
	MsgID      string `json:"msg_id"`       // 消息 ID
	Status     int    `json:"status"`       // 消息状态
	ServerTime int64  `json:"server_time"`  // 服务端时间戳
	Error      string `json:"error,omitempty"` // 错误信息
}

// WSPushMessage 服务端推送的消息
type WSPushMessage struct {
	MsgID      string `json:"msg_id"`       // 消息 ID
	FromUserID int64  `json:"from_user_id"` // 发送者用户 ID
	Content    string `json:"content"`      // 消息内容
	MsgType    int    `json:"msg_type"`     // 消息类型
	FileID     string `json:"file_id"`      // 文件ID（多媒体消息）
	Status     int    `json:"status"`       // 消息状态
	ClientTime int64  `json:"client_time"`  // 发送方的时间戳
	ServerTime int64  `json:"server_time"`  // 服务端时间戳
}

// WSStatusUpdate 消息状态更新
type WSStatusUpdate struct {
	MsgID      string `json:"msg_id"`       // 消息 ID
	Status     int    `json:"status"`       // 新状态
	UpdateTime int64  `json:"update_time"`  // 更新时间戳
}

// WSReceipt 回执（送达/已读）
type WSReceipt struct {
	MsgID string `json:"msg_id"` // 消息 ID
	Type  string `json:"type"`   // 回执类型（"delivered" 或 "read"）
	Time  int64  `json:"time"`   // 时间戳
}
