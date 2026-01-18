package service

import (
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/bbadbeef/go-base/user/internal/model"
	"github.com/bbadbeef/go-base/user/internal/repository"
)

// AuthService 认证服务
type AuthService struct {
	userRepo *repository.UserRepository
	codeRepo *repository.CodeRepository
}

// NewAuthService 创建认证服务
func NewAuthService(userRepo *repository.UserRepository, codeRepo *repository.CodeRepository) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		codeRepo: codeRepo,
	}
}

// Register 用户注册
func (s *AuthService) Register(req *model.RegisterRequest) (*model.User, error) {
	// 验证输入
	if err := s.validateRegisterInput(req); err != nil {
		return nil, err
	}

	// 检查用户名是否存在
	exists, err := s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("username already exists")
	}

	// 检查手机号是否存在
	exists, err = s.userRepo.ExistsByPhone(req.Phone)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("phone already exists")
	}

	// 如果提供了验证码，进行验证
	if req.Code != "" {
		if err := s.VerifyCode(req.Phone, req.Code, model.CodeTypeRegister); err != nil {
			return nil, fmt.Errorf("invalid verification code: %w", err)
		}
	}

	// 加密密码
	passwordHash, err := s.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password failed: %w", err)
	}

	// 创建用户
	now := model.NowMillis()
	user := &model.User{
		Username:     req.Username,
		Phone:        req.Phone,
		PasswordHash: passwordHash,
		Nickname:     req.Username, // 默认昵称为用户名
		Status:       model.UserStatusNormal,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login 密码登录
func (s *AuthService) Login(req *model.LoginRequest) (*model.User, error) {
	if req.Phone == "" {
		return nil, fmt.Errorf("phone is required")
	}

	// 获取用户
	user, err := s.userRepo.GetByPhone(req.Phone)
	if err != nil {
		return nil, fmt.Errorf("invalid phone or password")
	}

	// 检查用户状态
	if user.Status != model.UserStatusNormal {
		return nil, fmt.Errorf("user is disabled")
	}

	// 验证密码
	if err := s.verifyPassword(user.PasswordHash, req.Password); err != nil {
		return nil, fmt.Errorf("invalid phone or password")
	}

	return user, nil
}

// LoginWithCode 验证码登录
func (s *AuthService) LoginWithCode(phone, code string) (*model.User, error) {
	if phone == "" || code == "" {
		return nil, fmt.Errorf("phone and code are required")
	}

	// 验证验证码
	if err := s.VerifyCode(phone, code, model.CodeTypeLogin); err != nil {
		return nil, fmt.Errorf("invalid verification code: %w", err)
	}

	// 获取用户
	user, err := s.userRepo.GetByPhone(phone)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// 检查用户状态
	if user.Status != model.UserStatusNormal {
		return nil, fmt.Errorf("user is disabled")
	}

	return user, nil
}

// VerifyCode 验证验证码
func (s *AuthService) VerifyCode(phone, code string, codeType int) error {
	// 获取最新验证码
	latestCode, err := s.codeRepo.GetLatest(phone, codeType)
	if err != nil {
		return fmt.Errorf("verification code not found or expired")
	}

	// 检查状态
	if latestCode.Status != model.CodeStatusUnused {
		return fmt.Errorf("verification code already used")
	}

	// 检查是否过期
	now := model.NowMillis()
	if now > latestCode.ExpireAt {
		_ = s.codeRepo.MarkAsExpired(now)
		return fmt.Errorf("verification code expired")
	}

	// 验证码匹配
	if latestCode.Code != code {
		return fmt.Errorf("invalid verification code")
	}

	// 标记为已使用
	if err := s.codeRepo.MarkAsUsed(latestCode.ID); err != nil {
		return err
	}

	return nil
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(userID int64, oldPassword, newPassword string) error {
	// 获取用户
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := s.verifyPassword(user.PasswordHash, oldPassword); err != nil {
		return fmt.Errorf("invalid old password")
	}

	// 验证新密码
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	// 加密新密码
	newPasswordHash, err := s.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}

	// 更新密码
	return s.userRepo.UpdatePassword(userID, newPasswordHash)
}

// ResetPassword 重置密码（通过验证码）
func (s *AuthService) ResetPassword(req *model.ResetPasswordRequest) error {
	// 验证验证码
	if err := s.VerifyCode(req.Phone, req.Code, model.CodeTypeResetPassword); err != nil {
		return err
	}

	// 获取用户
	user, err := s.userRepo.GetByPhone(req.Phone)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// 验证新密码
	if err := s.validatePassword(req.NewPassword); err != nil {
		return err
	}

	// 加密新密码
	newPasswordHash, err := s.hashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash password failed: %w", err)
	}

	// 更新密码
	return s.userRepo.UpdatePassword(user.ID, newPasswordHash)
}

// hashPassword 加密密码
func (s *AuthService) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// verifyPassword 验证密码
func (s *AuthService) verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// validateRegisterInput 验证注册输入
func (s *AuthService) validateRegisterInput(req *model.RegisterRequest) error {
	if req.Username == "" {
		return fmt.Errorf("username is required")
	}

	if len(req.Username) < 3 || len(req.Username) > 20 {
		return fmt.Errorf("username length must be between 3 and 20")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(req.Username) {
		return fmt.Errorf("username can only contain letters, numbers and underscores")
	}

	if err := s.validatePhone(req.Phone); err != nil {
		return err
	}

	return s.validatePassword(req.Password)
}

// validatePhone 验证手机号
func (s *AuthService) validatePhone(phone string) error {
	if phone == "" {
		return fmt.Errorf("phone is required")
	}

	if !regexp.MustCompile(`^1[3-9]\d{9}$`).MatchString(phone) {
		return fmt.Errorf("invalid phone format")
	}

	return nil
}

// validatePassword 验证密码
func (s *AuthService) validatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}

	if len(password) < 6 || len(password) > 20 {
		return fmt.Errorf("password length must be between 6 and 20")
	}

	return nil
}

// SendVerificationCode 发送验证码（需要外部实现短信发送）
func (s *AuthService) SendVerificationCode(phone string, codeType int) (string, error) {
	// 验证手机号
	if err := s.validatePhone(phone); err != nil {
		return "", err
	}

	// 生成6位随机验证码
	code := s.generateCode()

	// 设置过期时间（5分钟）
	expireAt := time.Now().Add(5 * time.Minute).UnixMilli()

	// 保存验证码
	verificationCode := &model.VerificationCode{
		Phone:     phone,
		Code:      code,
		Type:      codeType,
		Status:    model.CodeStatusUnused,
		ExpireAt:  expireAt,
		CreatedAt: model.NowMillis(),
	}

	if err := s.codeRepo.Create(verificationCode); err != nil {
		return "", err
	}

	return code, nil
}

// generateCode 生成6位随机验证码
func (s *AuthService) generateCode() string {
	return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}
