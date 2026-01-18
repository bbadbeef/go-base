package repository

import (
	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/user/internal/model"
)

// DBVerificationCode 验证码数据库模型
type DBVerificationCode struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Phone     string `gorm:"type:varchar(20);index:idx_phone_type;not null"`
	Code      string `gorm:"type:varchar(10);not null"`
	Type      int    `gorm:"type:tinyint;index:idx_phone_type;not null"`
	Status    int    `gorm:"type:tinyint;default:0"`
	ExpireAt  int64  `gorm:"type:bigint;not null"`
	CreatedAt int64  `gorm:"index:idx_created_at;not null"`
}

func (DBVerificationCode) TableName() string {
	return "user_verification_codes"
}

// CodeRepository 验证码仓库
type CodeRepository struct {
	db *gorm.DB
}

// NewCodeRepository 创建验证码仓库
func NewCodeRepository(db *gorm.DB) *CodeRepository {
	return &CodeRepository{db: db}
}

// InitTable 初始化数据库表
func (r *CodeRepository) InitTable() error {
	return r.db.AutoMigrate(&DBVerificationCode{})
}

// Create 创建验证码
func (r *CodeRepository) Create(code *model.VerificationCode) error {
	dbCode := &DBVerificationCode{
		Phone:     code.Phone,
		Code:      code.Code,
		Type:      code.Type,
		Status:    code.Status,
		ExpireAt:  code.ExpireAt,
		CreatedAt: code.CreatedAt,
	}

	if err := r.db.Create(dbCode).Error; err != nil {
		return err
	}

	code.ID = dbCode.ID
	return nil
}

// GetLatest 获取最新的验证码
func (r *CodeRepository) GetLatest(phone string, codeType int) (*model.VerificationCode, error) {
	var dbCode DBVerificationCode
	if err := r.db.Where("phone = ? AND type = ?", phone, codeType).
		Order("created_at DESC").
		First(&dbCode).Error; err != nil {
		return nil, err
	}

	return &model.VerificationCode{
		ID:        dbCode.ID,
		Phone:     dbCode.Phone,
		Code:      dbCode.Code,
		Type:      dbCode.Type,
		Status:    dbCode.Status,
		ExpireAt:  dbCode.ExpireAt,
		CreatedAt: dbCode.CreatedAt,
	}, nil
}

// MarkAsUsed 标记为已使用
func (r *CodeRepository) MarkAsUsed(id int64) error {
	return r.db.Model(&DBVerificationCode{}).
		Where("id = ?", id).
		Update("status", model.CodeStatusUsed).Error
}

// MarkAsExpired 标记过期的验证码
func (r *CodeRepository) MarkAsExpired(now int64) error {
	return r.db.Model(&DBVerificationCode{}).
		Where("expire_at < ? AND status = ?", now, model.CodeStatusUnused).
		Update("status", model.CodeStatusExpired).Error
}
