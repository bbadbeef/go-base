package model

// 消息类型常量
const (
	MsgTypeText  = 1 // 文本消息
	MsgTypeImage = 2 // 图片消息
	MsgTypeVoice = 3 // 语音消息
	MsgTypeVideo = 4 // 视频消息
	MsgTypeFile  = 5 // 文件消息
)

// 消息状态常量
const (
	MsgStatusSending   = 1 // 发送中
	MsgStatusSent      = 2 // 已发送（服务端已接收）
	MsgStatusDelivered = 3 // 已送达（接收方已收到）
	MsgStatusRead      = 4 // 已读
	MsgStatusFailed    = 5 // 发送失败
)

// 会话类型常量
const (
	SessionTypeSingle = 1 // 单聊
	SessionTypeGroup  = 2 // 群聊
)

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	FromUserID int64  `json:"from_user_id"` // 发送者用户 ID（0 表示系统消息）
	ToUserID   int64  `json:"to_user_id"`   // 接收者用户 ID（单聊时使用）
	GroupID    int64  `json:"group_id"`     // 群组 ID（群聊时使用，单聊时为 0）
	Content    string `json:"content"`      // 消息内容
	MsgType    int    `json:"msg_type"`     // 消息类型（1:文本 2:图片 3:语音 4:视频 5:文件）
	FileID     string `json:"file_id"`      // 文件ID（多媒体消息时使用）
}

// Message 消息
type Message struct {
	MsgID         string                 `json:"msg_id"`                   // 消息唯一 ID
	FromUserID    int64                  `json:"from_user_id"`             // 发送者用户 ID
	ToUserID      int64                  `json:"to_user_id"`               // 接收者用户 ID
	GroupID       int64                  `json:"group_id"`                 // 群组 ID（0 表示单聊）
	Content       string                 `json:"content"`                  // 消息内容
	MsgType       int                    `json:"msg_type"`                 // 消息类型
	Status        int                    `json:"status"`                   // 消息状态
	FileID        string                 `json:"file_id,omitempty"`        // 文件ID（多媒体消息）
	FileInfo      *FileInfo              `json:"file_info,omitempty"`      // 文件信息（多媒体消息）
	ClientTime    int64                  `json:"client_time"`              // 客户端时间戳（毫秒）
	ServerTime    int64                  `json:"server_time"`              // 服务端时间戳（毫秒）
	DeliveredTime int64                  `json:"delivered_time"`           // 送达时间戳（毫秒）
	ReadTime      int64                  `json:"read_time"`                // 已读时间戳（毫秒）
}

// FileInfo 文件信息
type FileInfo struct {
	FileID   string `json:"file_id"`             // 文件ID
	FileName string `json:"file_name"`           // 文件名
	FileType string `json:"file_type"`           // 文件类型
	MimeType string `json:"mime_type"`           // MIME类型
	FileSize int64  `json:"file_size"`           // 文件大小
	FileURL  string `json:"file_url"`            // 文件访问URL
	Width    int    `json:"width,omitempty"`     // 宽度（图片/视频）
	Height   int    `json:"height,omitempty"`    // 高度（图片/视频）
	Duration int    `json:"duration,omitempty"`  // 时长（音频/视频）
}

// Session 会话
type Session struct {
	UserID         int64  `json:"user_id"`          // 用户 ID
	TargetID       int64  `json:"target_id"`        // 对方用户 ID 或群组 ID
	SessionType    int    `json:"session_type"`     // 会话类型（1:单聊 2:群聊）
	LastMsgContent string `json:"last_msg_content"` // 最后一条消息内容
	LastMsgTime    int64  `json:"last_msg_time"`    // 最后消息时间戳（毫秒）
	UnreadCount    int    `json:"unread_count"`     // 未读消息数
}

// GetMessagesRequest 获取历史消息请求
type GetMessagesRequest struct {
	UserID      int64 `json:"user_id"`       // 当前用户 ID
	TargetID    int64 `json:"target_id"`     // 对方用户 ID 或群组 ID
	SessionType int   `json:"session_type"`  // 会话类型（1:单聊 2:群聊）
	BeforeTime  int64 `json:"before_time"`   // 获取此时间之前的消息（分页），0 表示最新
	Limit       int   `json:"limit"`         // 每页条数
}

// Group 群组
type Group struct {
	GroupID   int64  `json:"group_id"`   // 群组 ID
	GroupName string `json:"group_name"` // 群组名称
	OwnerID   int64  `json:"owner_id"`   // 群主用户 ID
	AvatarURL string `json:"avatar_url"` // 群头像 URL
	CreatedAt int64  `json:"created_at"` // 创建时间戳（毫秒）
}

// GroupMember 群成员
type GroupMember struct {
	GroupID  int64 `json:"group_id"`  // 群组 ID
	UserID   int64 `json:"user_id"`   // 用户 ID
	Role     int   `json:"role"`      // 角色（0:普通成员 1:管理员 2:群主）
	JoinedAt int64 `json:"joined_at"` // 加入时间戳（毫秒）
}
