package repository

import (
	"strings"
	
	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/user/internal/model"
)

// DBUser 用户数据库模型
type DBUser struct {
	ID           int64   `gorm:"primaryKey;autoIncrement"`
	Username     string  `gorm:"type:varchar(50);uniqueIndex:uk_username;not null"`
	Phone        string  `gorm:"type:varchar(20);uniqueIndex:uk_phone;not null"`
	PasswordHash string  `gorm:"type:varchar(255);not null"`
	Nickname     string  `gorm:"type:varchar(50)"`
	Avatar       string  `gorm:"type:varchar(500)"`
	Email        string  `gorm:"type:varchar(100)"`
	Gender       int     `gorm:"type:tinyint;default:0"`
	Birthday     *string `gorm:"type:date"`
	Signature    string  `gorm:"type:varchar(255)"`
	Status       int     `gorm:"type:tinyint;default:1"`
	CreatedAt    int64   `gorm:"index:idx_created_at;not null"`
	UpdatedAt    int64   `gorm:"not null"`
}

func (DBUser) TableName() string {
	return "user_users"
}

// UserRepository 用户仓库
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓库
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// InitTable 初始化数据库表
func (r *UserRepository) InitTable() error {
	err := r.db.AutoMigrate(&DBUser{})
	// 忽略DROP不存在的索引/外键错误（GORM迁移的已知问题）
	if err != nil && (strings.Contains(err.Error(), "Can't DROP") || 
		strings.Contains(err.Error(), "check that column/key exists")) {
		return nil
	}
	return err
}

// Create 创建用户
func (r *UserRepository) Create(user *model.User) error {
	dbUser := &DBUser{
		Username:     user.Username,
		Phone:        user.Phone,
		PasswordHash: user.PasswordHash,
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		Email:        user.Email,
		Gender:       user.Gender,
		Birthday:     user.Birthday,
		Signature:    user.Signature,
		Status:       user.Status,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	if err := r.db.Create(dbUser).Error; err != nil {
		return err
	}

	user.ID = dbUser.ID
	return nil
}

// GetByID 根据 ID 获取用户
func (r *UserRepository) GetByID(id int64) (*model.User, error) {
	var dbUser DBUser
	if err := r.db.First(&dbUser, id).Error; err != nil {
		return nil, err
	}
	return r.toModel(&dbUser), nil
}

// GetByUsername 根据用户名获取用户
func (r *UserRepository) GetByUsername(username string) (*model.User, error) {
	var dbUser DBUser
	if err := r.db.Where("username = ?", username).First(&dbUser).Error; err != nil {
		return nil, err
	}
	return r.toModel(&dbUser), nil
}

// GetByPhone 根据手机号获取用户
func (r *UserRepository) GetByPhone(phone string) (*model.User, error) {
	var dbUser DBUser
	if err := r.db.Where("phone = ?", phone).First(&dbUser).Error; err != nil {
		return nil, err
	}
	return r.toModel(&dbUser), nil
}

// ExistsByUsername 检查用户名是否存在
func (r *UserRepository) ExistsByUsername(username string) (bool, error) {
	var count int64
	if err := r.db.Model(&DBUser{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByPhone 检查手机号是否存在
func (r *UserRepository) ExistsByPhone(phone string) (bool, error) {
	var count int64
	if err := r.db.Model(&DBUser{}).Where("phone = ?", phone).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update 更新用户信息
func (r *UserRepository) Update(user *model.User) error {
	dbUser := &DBUser{
		ID:           user.ID,
		Username:     user.Username,
		Phone:        user.Phone,
		PasswordHash: user.PasswordHash,
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		Email:        user.Email,
		Gender:       user.Gender,
		Birthday:     user.Birthday,
		Signature:    user.Signature,
		Status:       user.Status,
		UpdatedAt:    user.UpdatedAt,
	}
	return r.db.Save(dbUser).Error
}

// UpdatePassword 更新密码
func (r *UserRepository) UpdatePassword(userID int64, passwordHash string) error {
	return r.db.Model(&DBUser{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"password_hash": passwordHash,
			"updated_at":    model.NowMillis(),
		}).Error
}

// toModel 转换为业务模型
func (r *UserRepository) toModel(dbUser *DBUser) *model.User {
	return &model.User{
		ID:           dbUser.ID,
		Username:     dbUser.Username,
		Phone:        dbUser.Phone,
		PasswordHash: dbUser.PasswordHash,
		Nickname:     dbUser.Nickname,
		Avatar:       dbUser.Avatar,
		Email:        dbUser.Email,
		Gender:       dbUser.Gender,
		Birthday:     dbUser.Birthday,
		Signature:    dbUser.Signature,
		Status:       dbUser.Status,
		CreatedAt:    dbUser.CreatedAt,
		UpdatedAt:    dbUser.UpdatedAt,
	}
}
