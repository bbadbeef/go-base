package repository

import (
	"strings"
	
	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/im/internal/model"
)

// DBMessage 消息数据库模型
type DBMessage struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	MsgID         string `gorm:"type:varchar(64);uniqueIndex:uk_msg_id;not null"`
	FromUserID    int64  `gorm:"index:idx_from;not null"`
	ToUserID      int64  `gorm:"index:idx_to;not null"`
	GroupID       int64  `gorm:"index:idx_group;default:0"`
	Content       string `gorm:"type:text;not null"`
	MsgType       int    `gorm:"type:tinyint;default:1"`
	Status        int    `gorm:"type:tinyint;default:1"`
	FileID        string `gorm:"type:varchar(64);index:idx_file_id"` // 文件ID（多媒体消息）
	ClientTime    int64  `gorm:"type:bigint"`
	ServerTime    int64  `gorm:"type:bigint;index:idx_server_time;not null"`
	DeliveredTime int64  `gorm:"type:bigint;default:0"`
	ReadTime      int64  `gorm:"type:bigint;default:0"`
	CreatedAt     int64  `gorm:"autoCreateTime:milli"`
}

func (DBMessage) TableName() string {
	return "im_messages"
}

// MessageRepository 消息仓库
type MessageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓库
func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// InitTables 初始化数据库表
func (r *MessageRepository) InitTables() error {
	// 自动迁移消息表
	err := r.db.AutoMigrate(&DBMessage{})
	// 忽略DROP不存在的索引/外键错误（GORM迁移的已知问题）
	if err != nil && (strings.Contains(err.Error(), "Can't DROP") || 
		strings.Contains(err.Error(), "check that column/key exists")) {
		err = nil
	}
	if err != nil {
		return err
	}

	// 创建复合索引（MySQL 需要先检查是否存在）
	// 检查并创建 idx_to_status_time 索引
	var count int64
	r.db.Raw(`
		SELECT COUNT(1) 
		FROM information_schema.statistics 
		WHERE table_schema = DATABASE() 
		AND table_name = 'im_messages' 
		AND index_name = 'idx_to_status_time'
	`).Scan(&count)
	
	if count == 0 {
		r.db.Exec(`CREATE INDEX idx_to_status_time ON im_messages(to_user_id, status, server_time DESC)`)
	}

	// 检查并创建 idx_server_time_id 索引
	r.db.Raw(`
		SELECT COUNT(1) 
		FROM information_schema.statistics 
		WHERE table_schema = DATABASE() 
		AND table_name = 'im_messages' 
		AND index_name = 'idx_server_time_id'
	`).Scan(&count)
	
	if count == 0 {
		r.db.Exec(`CREATE INDEX idx_server_time_id ON im_messages(server_time DESC, id DESC)`)
	}

	return nil
}

// Save 保存消息
func (r *MessageRepository) Save(msg *model.Message) error {
	dbMsg := &DBMessage{
		MsgID:         msg.MsgID,
		FromUserID:    msg.FromUserID,
		ToUserID:      msg.ToUserID,
		GroupID:       msg.GroupID,
		Content:       msg.Content,
		MsgType:       msg.MsgType,
		Status:        msg.Status,
		FileID:        msg.FileID,
		ClientTime:    msg.ClientTime,
		ServerTime:    msg.ServerTime,
		DeliveredTime: msg.DeliveredTime,
		ReadTime:      msg.ReadTime,
	}
	return r.db.Create(dbMsg).Error
}

// GetByMsgID 根据消息 ID 查询
func (r *MessageRepository) GetByMsgID(msgID string) (*model.Message, error) {
	var dbMsg DBMessage
	if err := r.db.Where("msg_id = ?", msgID).First(&dbMsg).Error; err != nil {
		return nil, err
	}
	return r.toModel(&dbMsg), nil
}

// UpdateStatus 更新消息状态
func (r *MessageRepository) UpdateStatus(msgID string, status int, updateTime int64) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == model.MsgStatusDelivered {
		updates["delivered_time"] = updateTime
	} else if status == model.MsgStatusRead {
		updates["read_time"] = updateTime
	}

	return r.db.Model(&DBMessage{}).Where("msg_id = ?", msgID).Updates(updates).Error
}

// GetMessages 获取历史消息
func (r *MessageRepository) GetMessages(req *model.GetMessagesRequest) ([]*model.Message, error) {
	var dbMessages []DBMessage

	query := r.db.Model(&DBMessage{})

	// 单聊消息查询
	if req.SessionType == model.SessionTypeSingle {
		query = query.Where(
			"(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
			req.UserID, req.TargetID, req.TargetID, req.UserID,
		)
	} else {
		// 群聊消息查询
		query = query.Where("group_id = ?", req.TargetID)
	}

	// 分页查询
	if req.BeforeTime > 0 {
		query = query.Where("server_time < ?", req.BeforeTime)
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	if err := query.Order("server_time DESC").Limit(req.Limit).Find(&dbMessages).Error; err != nil {
		return nil, err
	}

	// 转换为模型
	messages := make([]*model.Message, len(dbMessages))
	for i, dbMsg := range dbMessages {
		messages[i] = r.toModel(&dbMsg)
	}

	return messages, nil
}

// GetUndeliveredMessages 获取未送达消息
func (r *MessageRepository) GetUndeliveredMessages(userID int64, limit int) ([]*model.Message, error) {
	var dbMessages []DBMessage

	if err := r.db.Where("to_user_id = ? AND status = ?", userID, model.MsgStatusSent).
		Order("server_time ASC").
		Limit(limit).
		Find(&dbMessages).Error; err != nil {
		return nil, err
	}

	messages := make([]*model.Message, len(dbMessages))
	for i, dbMsg := range dbMessages {
		messages[i] = r.toModel(&dbMsg)
	}

	return messages, nil
}

// toModel 转换为业务模型
func (r *MessageRepository) toModel(dbMsg *DBMessage) *model.Message {
	return &model.Message{
		MsgID:         dbMsg.MsgID,
		FromUserID:    dbMsg.FromUserID,
		ToUserID:      dbMsg.ToUserID,
		GroupID:       dbMsg.GroupID,
		Content:       dbMsg.Content,
		MsgType:       dbMsg.MsgType,
		Status:        dbMsg.Status,
		FileID:        dbMsg.FileID,
		ClientTime:    dbMsg.ClientTime,
		ServerTime:    dbMsg.ServerTime,
		DeliveredTime: dbMsg.DeliveredTime,
		ReadTime:      dbMsg.ReadTime,
	}
}
