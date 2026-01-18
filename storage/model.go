package storage

import (
	"time"
)

// DBFile 文件数据库模型
type DBFile struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	FileID    string    `gorm:"type:varchar(64);uniqueIndex:uk_file_id;not null"`
	UserID    int64     `gorm:"index:idx_user;not null"`
	FileName  string    `gorm:"type:varchar(255);not null"`
	FileType  string    `gorm:"type:varchar(50);not null;index:idx_type"`
	MimeType  string    `gorm:"type:varchar(100);not null"`
	FileSize  int64     `gorm:"not null"`
	FileData  []byte    `gorm:"type:mediumblob;not null"` // 最大 16MB
	Width     int       `gorm:"type:int;default:0"`
	Height    int       `gorm:"type:int;default:0"`
	Duration  int       `gorm:"type:int;default:0"`
	Status    int       `gorm:"type:tinyint;default:1;index:idx_status"` // 1:正常 2:已删除
	CreatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created"`
}

func (DBFile) TableName() string {
	return "storage_files"
}
