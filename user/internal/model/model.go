package model

import "time"

// User 用户模型
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"` // 不返回给前端
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	Email        string    `json:"email"`
	Gender       int       `json:"gender"`        // 0-未知，1-男，2-女
	Birthday     *string   `json:"birthday"`      // YYYY-MM-DD
	Signature    string    `json:"signature"`
	Status       int       `json:"status"`        // 0-禁用，1-正常
	CreatedAt    int64     `json:"created_at"`    // 毫秒时间戳
	UpdatedAt    int64     `json:"updated_at"`
}

// UserProfile 用户公开信息（不包含敏感信息）
type UserProfile struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Gender    int    `json:"gender"`
	Signature string `json:"signature"`
}

// VerificationCode 验证码模型
type VerificationCode struct {
	ID        int64  `json:"id"`
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	Type      int    `json:"type"`      // 1-注册，2-登录，3-重置密码
	Status    int    `json:"status"`    // 0-未使用，1-已使用，2-已过期
	ExpireAt  int64  `json:"expire_at"` // 过期时间(毫秒)
	CreatedAt int64  `json:"created_at"`
}

// 验证码类型
const (
	CodeTypeRegister      = 1
	CodeTypeLogin         = 2
	CodeTypeResetPassword = 3
)

// 验证码状态
const (
	CodeStatusUnused  = 0
	CodeStatusUsed    = 1
	CodeStatusExpired = 2
)

// 用户状态
const (
	UserStatusDisabled = 0
	UserStatusNormal   = 1
)

// 性别
const (
	GenderUnknown = 0
	GenderMale    = 1
	GenderFemale  = 2
)

// RegisterRequest 注册请求
type RegisterRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password,omitempty"` // 密码（密码注册时使用）
	Code     string `json:"code,omitempty"`     // 验证码（验证码注册时使用）
}

// LoginRequest 登录请求
type LoginRequest struct {
	Account  string `json:"account"`            // 账号：手机号或用户名
	Password string `json:"password,omitempty"` // 密码登录时使用
	Code     string `json:"code,omitempty"`     // 验证码登录时使用（仅手机号）
}

// UpdateProfileRequest 更新用户信息请求
type UpdateProfileRequest struct {
	Nickname  *string `json:"nickname,omitempty"`
	Avatar    *string `json:"avatar,omitempty"`
	Email     *string `json:"email,omitempty"`
	Gender    *int    `json:"gender,omitempty"`
	Birthday  *string `json:"birthday,omitempty"`  // YYYY-MM-DD
	Signature *string `json:"signature,omitempty"`
}

// SendCodeRequest 发送验证码请求
type SendCodeRequest struct {
	Phone string `json:"phone"`
	Type  int    `json:"type"` // 1-注册，2-登录，3-重置密码
}

// VerifyCodeRequest 验证验证码请求
type VerifyCodeRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
	Type  int    `json:"type"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

// ToProfile 转换为公开信息
func (u *User) ToProfile() *UserProfile {
	return &UserProfile{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Avatar:    u.Avatar,
		Gender:    u.Gender,
		Signature: u.Signature,
	}
}

// NowMillis 获取当前毫秒时间戳
func NowMillis() int64 {
	return time.Now().UnixMilli()
}
