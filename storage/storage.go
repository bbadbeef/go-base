// Package storage 提供文件存储功能
// 支持将文件存储到数据库，适用于小文件（<10MB）的多节点部署场景
package storage

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 文件类型常量
const (
	FileTypeImage = "image" // 图片
	FileTypeVideo = "video" // 视频
	FileTypeVoice = "voice" // 语音
	FileTypeFile  = "file"  // 普通文件
)

// 文件大小限制（字节）
const (
	MaxImageSize = 10 * 1024 * 1024  // 10MB
	MaxVideoSize = 10 * 1024 * 1024  // 10MB
	MaxVoiceSize = 10 * 1024 * 1024  // 10MB
	MaxFileSize  = 10 * 1024 * 1024  // 10MB
)

// 允许的文件类型
var (
	AllowedImageTypes = []string{
		"image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp", "image/bmp",
	}
	AllowedVideoTypes = []string{
		"video/mp4", "video/quicktime", "video/x-msvideo", "video/mpeg",
	}
	AllowedVoiceTypes = []string{
		"audio/mpeg", "audio/mp3", "audio/wav", "audio/ogg", "audio/aac", "audio/mp4",
	}
)

// FileInfo 文件信息
type FileInfo struct {
	FileID     string                 `json:"file_id"`              // 文件唯一ID
	FileName   string                 `json:"file_name"`            // 原始文件名
	FileType   string                 `json:"file_type"`            // 文件类型（image/video/voice/file）
	MimeType   string                 `json:"mime_type"`            // MIME类型
	FileSize   int64                  `json:"file_size"`            // 文件大小（字节）
	Width      int                    `json:"width,omitempty"`      // 宽度（图片/视频）
	Height     int                    `json:"height,omitempty"`     // 高度（图片/视频）
	Duration   int                    `json:"duration,omitempty"`   // 时长（音频/视频，秒）
	URL        string                 `json:"url"`                  // 访问URL
	Thumbnail  string                 `json:"thumbnail,omitempty"`  // 缩略图URL（图片/视频）
	ExtraData  map[string]interface{} `json:"extra_data,omitempty"` // 扩展数据
	UploadTime time.Time              `json:"upload_time"`          // 上传时间
}

// UploadRequest 上传请求
type UploadRequest struct {
	File     multipart.File   // 文件
	Header   *multipart.FileHeader // 文件头信息
	UserID   int64            // 上传用户ID
	FileType string           // 文件类型
}

// Storage 存储接口
type Storage interface {
	// Upload 上传文件
	Upload(req *UploadRequest) (*FileInfo, error)

	// Download 下载文件
	Download(fileID string) ([]byte, *FileInfo, error)

	// GetFileInfo 获取文件信息
	GetFileInfo(fileID string) (*FileInfo, error)

	// Delete 删除文件
	Delete(fileID string) error

	// DeleteByUser 删除用户的所有文件
	DeleteByUser(userID int64) error
}

// Config 存储配置
type Config struct {
	DB      *gorm.DB // 数据库连接
	BaseURL string   // 文件访问基础URL，如 "http://localhost:8080"
}

// dbStorage 数据库存储实现
type dbStorage struct {
	db      *gorm.DB
	baseURL string
}

// NewStorage 创建存储实例
func NewStorage(config *Config) (Storage, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	storage := &dbStorage{
		db:      config.DB,
		baseURL: strings.TrimSuffix(config.BaseURL, "/"),
	}

	// 初始化数据库表
	if err := storage.initTable(); err != nil {
		return nil, fmt.Errorf("init storage table failed: %w", err)
	}

	return storage, nil
}

// initTable 初始化数据库表
func (s *dbStorage) initTable() error {
	err := s.db.AutoMigrate(&DBFile{})
	// 忽略DROP不存在的索引/外键错误（GORM迁移的已知问题）
	if err != nil && (strings.Contains(err.Error(), "Can't DROP") || 
		strings.Contains(err.Error(), "check that column/key exists")) {
		return nil
	}
	return err
}

// Upload 上传文件
func (s *dbStorage) Upload(req *UploadRequest) (*FileInfo, error) {
	if req == nil || req.File == nil || req.Header == nil {
		return nil, fmt.Errorf("invalid upload request")
	}

	// 读取文件内容
	data, err := io.ReadAll(req.File)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	// 获取文件信息
	fileName := req.Header.Filename
	fileSize := int64(len(data))
	mimeType := req.Header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = detectMimeType(fileName, data)
	}

	// 验证文件
	if err := s.validateFile(req.FileType, mimeType, fileSize); err != nil {
		return nil, err
	}

	// 生成文件ID
	fileID := generateFileID()

	// 创建数据库记录
	dbFile := &DBFile{
		FileID:   fileID,
		UserID:   req.UserID,
		FileName: fileName,
		FileType: req.FileType,
		MimeType: mimeType,
		FileSize: fileSize,
		FileData: data,
		Status:   1, // 正常
	}

	// 保存到数据库
	if err := s.db.Create(dbFile).Error; err != nil {
		return nil, fmt.Errorf("save file to database failed: %w", err)
	}

	// 构建文件信息
	fileInfo := &FileInfo{
		FileID:     fileID,
		FileName:   fileName,
		FileType:   req.FileType,
		MimeType:   mimeType,
		FileSize:   fileSize,
		URL:        fmt.Sprintf("%s/api/files/%s", s.baseURL, fileID),
		UploadTime: dbFile.CreatedAt,
	}

	return fileInfo, nil
}

// Download 下载文件
func (s *dbStorage) Download(fileID string) ([]byte, *FileInfo, error) {
	var dbFile DBFile
	if err := s.db.Where("file_id = ? AND status = 1", fileID).First(&dbFile).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("file not found")
		}
		return nil, nil, err
	}

	fileInfo := &FileInfo{
		FileID:     dbFile.FileID,
		FileName:   dbFile.FileName,
		FileType:   dbFile.FileType,
		MimeType:   dbFile.MimeType,
		FileSize:   dbFile.FileSize,
		URL:        fmt.Sprintf("%s/api/files/%s", s.baseURL, dbFile.FileID),
		UploadTime: dbFile.CreatedAt,
	}

	return dbFile.FileData, fileInfo, nil
}

// GetFileInfo 获取文件信息
func (s *dbStorage) GetFileInfo(fileID string) (*FileInfo, error) {
	var dbFile DBFile
	if err := s.db.Select("file_id, user_id, file_name, file_type, mime_type, file_size, created_at").
		Where("file_id = ? AND status = 1", fileID).First(&dbFile).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("file not found")
		}
		return nil, err
	}

	return &FileInfo{
		FileID:     dbFile.FileID,
		FileName:   dbFile.FileName,
		FileType:   dbFile.FileType,
		MimeType:   dbFile.MimeType,
		FileSize:   dbFile.FileSize,
		URL:        fmt.Sprintf("%s/api/files/%s", s.baseURL, dbFile.FileID),
		UploadTime: dbFile.CreatedAt,
	}, nil
}

// Delete 删除文件
func (s *dbStorage) Delete(fileID string) error {
	result := s.db.Model(&DBFile{}).
		Where("file_id = ?", fileID).
		Update("status", 2) // 标记为已删除

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("file not found")
	}

	return nil
}

// DeleteByUser 删除用户的所有文件
func (s *dbStorage) DeleteByUser(userID int64) error {
	return s.db.Model(&DBFile{}).
		Where("user_id = ?", userID).
		Update("status", 2).Error
}

// validateFile 验证文件
func (s *dbStorage) validateFile(fileType, mimeType string, fileSize int64) error {
	// 检查文件大小
	var maxSize int64
	switch fileType {
	case FileTypeImage:
		maxSize = MaxImageSize
		if !isAllowedMimeType(mimeType, AllowedImageTypes) {
			return fmt.Errorf("不支持的图片格式: %s", mimeType)
		}
	case FileTypeVideo:
		maxSize = MaxVideoSize
		if !isAllowedMimeType(mimeType, AllowedVideoTypes) {
			return fmt.Errorf("不支持的视频格式: %s", mimeType)
		}
	case FileTypeVoice:
		maxSize = MaxVoiceSize
		if !isAllowedMimeType(mimeType, AllowedVoiceTypes) {
			return fmt.Errorf("不支持的语音格式: %s", mimeType)
		}
	case FileTypeFile:
		maxSize = MaxFileSize
	default:
		return fmt.Errorf("未知的文件类型: %s", fileType)
	}

	if fileSize > maxSize {
		return fmt.Errorf("文件大小超过限制，最大 %.1fMB", float64(maxSize)/(1024*1024))
	}

	return nil
}

// generateFileID 生成文件ID
func generateFileID() string {
	return uuid.New().String()
}

// isAllowedMimeType 检查是否允许的MIME类型
func isAllowedMimeType(mimeType string, allowedTypes []string) bool {
	mimeType = strings.ToLower(mimeType)
	for _, allowed := range allowedTypes {
		if strings.HasPrefix(mimeType, allowed) {
			return true
		}
	}
	return false
}

// detectMimeType 检测MIME类型
func detectMimeType(fileName string, data []byte) string {
	// 根据文件扩展名判断
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	}

	// 根据文件头魔数判断
	if len(data) >= 4 {
		if bytes.Equal(data[0:2], []byte{0xFF, 0xD8}) {
			return "image/jpeg"
		}
		if bytes.Equal(data[0:4], []byte{0x89, 0x50, 0x4E, 0x47}) {
			return "image/png"
		}
		if bytes.Equal(data[0:4], []byte{0x47, 0x49, 0x46, 0x38}) {
			return "image/gif"
		}
	}

	return "application/octet-stream"
}
