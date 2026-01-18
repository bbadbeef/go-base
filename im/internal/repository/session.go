package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bbadbeef/go-base/im/internal/model"
)

// DBSession 会话数据库模型
type DBSession struct {
	UserID         int64  `gorm:"primaryKey"`
	TargetID       int64  `gorm:"primaryKey"`
	SessionType    int    `gorm:"primaryKey;type:tinyint;default:1"`
	LastMsgContent string `gorm:"type:text"`
	LastMsgTime    int64  `gorm:"type:bigint;index:idx_user_time"`
	UnreadCount    int    `gorm:"type:int;default:0"`
	CreatedAt      int64  `gorm:"autoCreateTime:milli"`
	UpdatedAt      int64  `gorm:"autoUpdateTime:milli"`
}

func (DBSession) TableName() string {
	return "im_sessions"
}

// SessionRepository 会话仓库
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository 创建会话仓库
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// InitTables 初始化数据库表
func (r *SessionRepository) InitTables() error {
	return r.db.AutoMigrate(&DBSession{})
}

// UpdateSession 更新会话（如果不存在则创建）
func (r *SessionRepository) UpdateSession(session *model.Session) error {
	dbSession := &DBSession{
		UserID:         session.UserID,
		TargetID:       session.TargetID,
		SessionType:    session.SessionType,
		LastMsgContent: session.LastMsgContent,
		LastMsgTime:    session.LastMsgTime,
	}

	// 使用 upsert 模式
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "target_id"},
			{Name: "session_type"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_msg_content": session.LastMsgContent,
			"last_msg_time":    session.LastMsgTime,
			"unread_count":     gorm.Expr("unread_count + ?", session.UnreadCount),
		}),
	}).Create(dbSession).Error
}

// GetUserSessions 获取用户的会话列表
func (r *SessionRepository) GetUserSessions(userID int64) ([]*model.Session, error) {
	var dbSessions []DBSession

	if err := r.db.Where("user_id = ?", userID).
		Order("last_msg_time DESC").
		Find(&dbSessions).Error; err != nil {
		return nil, err
	}

	sessions := make([]*model.Session, len(dbSessions))
	for i, s := range dbSessions {
		sessions[i] = &model.Session{
			UserID:         s.UserID,
			TargetID:       s.TargetID,
			SessionType:    s.SessionType,
			LastMsgContent: s.LastMsgContent,
			LastMsgTime:    s.LastMsgTime,
			UnreadCount:    s.UnreadCount,
		}
	}

	return sessions, nil
}

// ClearUnread 清除未读数
func (r *SessionRepository) ClearUnread(userID, targetID int64, sessionType int) error {
	return r.db.Model(&DBSession{}).
		Where("user_id = ? AND target_id = ? AND session_type = ?", userID, targetID, sessionType).
		Update("unread_count", 0).Error
}
