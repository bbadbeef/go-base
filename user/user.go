// Package user 提供用户管理功能
// 支持注册、登录、验证码认证、用户信息管理
package user

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/user/internal/jwt"
	"github.com/bbadbeef/go-base/user/internal/model"
	"github.com/bbadbeef/go-base/user/internal/repository"
	"github.com/bbadbeef/go-base/user/internal/service"
)

// 重新导出类型给外部使用
type (
	User                   = model.User
	UserProfile            = model.UserProfile
	RegisterRequest        = model.RegisterRequest
	LoginRequest           = model.LoginRequest
	UpdateProfileRequest   = model.UpdateProfileRequest
	SendCodeRequest        = model.SendCodeRequest
	VerifyCodeRequest      = model.VerifyCodeRequest
	ChangePasswordRequest  = model.ChangePasswordRequest
	ResetPasswordRequest   = model.ResetPasswordRequest
	JWTClaims              = jwt.Claims
)

// 重新导出常量
const (
	CodeTypeRegister      = model.CodeTypeRegister
	CodeTypeLogin         = model.CodeTypeLogin
	CodeTypeResetPassword = model.CodeTypeResetPassword

	UserStatusDisabled = model.UserStatusDisabled
	UserStatusNormal   = model.UserStatusNormal

	GenderUnknown = model.GenderUnknown
	GenderMale    = model.GenderMale
	GenderFemale  = model.GenderFemale
)

// Config 用户模块配置
type Config struct {
	DB            *gorm.DB       // 数据库连接
	JWTSecret     string         // JWT密钥
	TokenDuration time.Duration  // Token有效期，默认7天
}

// Service 用户服务接口
type Service interface {
	// 认证相关
	Register(req *RegisterRequest) (*User, string, error)
	Login(req *LoginRequest) (*User, string, error)
	LoginWithCode(phone, code string) (*User, string, error)
	ChangePassword(userID int64, req *ChangePasswordRequest) error
	ResetPassword(req *ResetPasswordRequest) error

	// 验证码相关
	SendVerificationCode(req *SendCodeRequest) (string, error)
	VerifyCode(req *VerifyCodeRequest) error

	// 用户信息相关
	GetUserByID(id int64) (*User, error)
	GetUserProfile(id int64) (*UserProfile, error)
	UpdateProfile(userID int64, req *UpdateProfileRequest) (*User, error)

	// JWT相关
	ValidateToken(token string) (*JWTClaims, error)
	RefreshToken(token string) (string, error)
}

// userService 用户服务实现
type userService struct {
	authService *service.AuthService
	userService *service.UserService
	jwtManager  *jwt.JWTManager
}

// NewService 创建用户服务实例
func NewService(config *Config) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	if config.JWTSecret == "" {
		return nil, fmt.Errorf("JWT secret is required")
	}

	// 设置默认token有效期
	if config.TokenDuration == 0 {
		config.TokenDuration = 7 * 24 * time.Hour // 7天
	}

	// 初始化仓库层
	userRepo := repository.NewUserRepository(config.DB)
	codeRepo := repository.NewCodeRepository(config.DB)

	// 自动创建表
	if err := userRepo.InitTable(); err != nil {
		return nil, fmt.Errorf("init user table failed: %w", err)
	}
	if err := codeRepo.InitTable(); err != nil {
		return nil, fmt.Errorf("init code table failed: %w", err)
	}

	// 初始化服务层
	authService := service.NewAuthService(userRepo, codeRepo)
	userSvc := service.NewUserService(userRepo)

	// 初始化JWT管理器
	jwtMgr := jwt.NewJWTManager(config.JWTSecret, config.TokenDuration)

	return &userService{
		authService: authService,
		userService: userSvc,
		jwtManager:  jwtMgr,
	}, nil
}

// Register 用户注册
func (s *userService) Register(req *RegisterRequest) (*User, string, error) {
	user, err := s.authService.Register(req)
	if err != nil {
		return nil, "", err
	}

	// 生成token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Username, user.Phone)
	if err != nil {
		return nil, "", fmt.Errorf("generate token failed: %w", err)
	}

	return user, token, nil
}

// Login 密码登录
func (s *userService) Login(req *LoginRequest) (*User, string, error) {
	user, err := s.authService.Login(req)
	if err != nil {
		return nil, "", err
	}

	// 生成token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Username, user.Phone)
	if err != nil {
		return nil, "", fmt.Errorf("generate token failed: %w", err)
	}

	return user, token, nil
}

// LoginWithCode 验证码登录
func (s *userService) LoginWithCode(phone, code string) (*User, string, error) {
	user, err := s.authService.LoginWithCode(phone, code)
	if err != nil {
		return nil, "", err
	}

	// 生成token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Username, user.Phone)
	if err != nil {
		return nil, "", fmt.Errorf("generate token failed: %w", err)
	}

	return user, token, nil
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(userID int64, req *ChangePasswordRequest) error {
	return s.authService.ChangePassword(userID, req.OldPassword, req.NewPassword)
}

// ResetPassword 重置密码
func (s *userService) ResetPassword(req *ResetPasswordRequest) error {
	return s.authService.ResetPassword(req)
}

// SendVerificationCode 发送验证码
func (s *userService) SendVerificationCode(req *SendCodeRequest) (string, error) {
	return s.authService.SendVerificationCode(req.Phone, req.Type)
}

// VerifyCode 验证验证码
func (s *userService) VerifyCode(req *VerifyCodeRequest) error {
	return s.authService.VerifyCode(req.Phone, req.Code, req.Type)
}

// GetUserByID 根据ID获取用户
func (s *userService) GetUserByID(id int64) (*User, error) {
	return s.userService.GetUserByID(id)
}

// GetUserProfile 获取用户公开信息
func (s *userService) GetUserProfile(id int64) (*UserProfile, error) {
	return s.userService.GetUserProfile(id)
}

// UpdateProfile 更新用户信息
func (s *userService) UpdateProfile(userID int64, req *UpdateProfileRequest) (*User, error) {
	return s.userService.UpdateProfile(userID, req)
}

// ValidateToken 验证token
func (s *userService) ValidateToken(token string) (*JWTClaims, error) {
	return s.jwtManager.ValidateToken(token)
}

// RefreshToken 刷新token
func (s *userService) RefreshToken(token string) (string, error) {
	return s.jwtManager.RefreshToken(token)
}
