package repository

import (
	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/im/internal/model"
)

// DBGroup 群组数据库模型
type DBGroup struct {
	GroupID   int64  `gorm:"primaryKey;autoIncrement"`
	GroupName string `gorm:"type:varchar(100);not null"`
	OwnerID   int64  `gorm:"not null"`
	AvatarURL string `gorm:"type:varchar(255)"`
	CreatedAt int64  `gorm:"autoCreateTime:milli"`
	UpdatedAt int64  `gorm:"autoUpdateTime:milli"`
}

func (DBGroup) TableName() string {
	return "im_groups"
}

// DBGroupMember 群成员数据库模型
type DBGroupMember struct {
	ID       int64 `gorm:"primaryKey;autoIncrement"`
	GroupID  int64 `gorm:"uniqueIndex:uk_group_user;index:idx_group;not null"`
	UserID   int64 `gorm:"uniqueIndex:uk_group_user;index:idx_user;not null"`
	Role     int   `gorm:"type:tinyint;default:0"`
	JoinedAt int64 `gorm:"autoCreateTime:milli"`
}

func (DBGroupMember) TableName() string {
	return "im_group_members"
}

// GroupRepository 群组仓库
type GroupRepository struct {
	db *gorm.DB
}

// NewGroupRepository 创建群组仓库
func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// InitTables 初始化数据库表
func (r *GroupRepository) InitTables() error {
	if err := r.db.AutoMigrate(&DBGroup{}, &DBGroupMember{}); err != nil {
		return err
	}
	return nil
}

// CreateGroup 创建群组
func (r *GroupRepository) CreateGroup(group *model.Group) error {
	dbGroup := &DBGroup{
		GroupName: group.GroupName,
		OwnerID:   group.OwnerID,
		AvatarURL: group.AvatarURL,
	}

	if err := r.db.Create(dbGroup).Error; err != nil {
		return err
	}

	group.GroupID = dbGroup.GroupID
	group.CreatedAt = dbGroup.CreatedAt
	return nil
}

// GetGroup 获取群组信息
func (r *GroupRepository) GetGroup(groupID int64) (*model.Group, error) {
	var dbGroup DBGroup
	if err := r.db.First(&dbGroup, groupID).Error; err != nil {
		return nil, err
	}

	return &model.Group{
		GroupID:   dbGroup.GroupID,
		GroupName: dbGroup.GroupName,
		OwnerID:   dbGroup.OwnerID,
		AvatarURL: dbGroup.AvatarURL,
		CreatedAt: dbGroup.CreatedAt,
	}, nil
}

// AddMember 添加群成员
func (r *GroupRepository) AddMember(member *model.GroupMember) error {
	dbMember := &DBGroupMember{
		GroupID: member.GroupID,
		UserID:  member.UserID,
		Role:    member.Role,
	}
	return r.db.Create(dbMember).Error
}

// RemoveMember 移除群成员
func (r *GroupRepository) RemoveMember(groupID, userID int64) error {
	return r.db.Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&DBGroupMember{}).Error
}

// GetMembers 获取群成员列表
func (r *GroupRepository) GetMembers(groupID int64) ([]*model.GroupMember, error) {
	var dbMembers []DBGroupMember
	if err := r.db.Where("group_id = ?", groupID).Find(&dbMembers).Error; err != nil {
		return nil, err
	}

	members := make([]*model.GroupMember, len(dbMembers))
	for i, m := range dbMembers {
		members[i] = &model.GroupMember{
			GroupID:  m.GroupID,
			UserID:   m.UserID,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		}
	}

	return members, nil
}

// IsMember 检查用户是否是群成员
func (r *GroupRepository) IsMember(groupID, userID int64) (bool, error) {
	var count int64
	if err := r.db.Model(&DBGroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
