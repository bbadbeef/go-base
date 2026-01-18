# User 用户管理模块

提供完整的用户管理功能，包括注册、登录、验证码认证、用户信息管理等。

## 功能特性

- ✅ 用户注册（支持密码注册或验证码注册）
- ✅ 密码登录（支持手机号或用户名）
- ✅ 验证码登录（仅支持手机号）
- ✅ 自动生成随机昵称（user_开头）
- ✅ 用户信息管理（头像、昵称、签名等）
- ✅ 修改密码
- ✅ 重置密码（通过验证码）
- ✅ JWT Token 认证
- ✅ Token 刷新
- ✅ 密码加密（bcrypt）

## 快速开始

### 1. 安装

```bash
go get github.com/bbadbeef/go-base/user
```

### 2. 创建数据库表

```bash
mysql -u root -p < sql/schema.sql
```

### 3. 使用示例

```go
package main

import (
    "database/sql"
    "time"
    
    _ "github.com/go-sql-driver/mysql"
    "github.com/bbadbeef/go-base/user"
)

func main() {
    // 连接数据库
    db, _ := sql.Open("mysql", "root:password@tcp(localhost:3306)/mydb?parseTime=true")
    
    // 创建用户服务
    userService, _ := user.NewService(&user.Config{
        DB:            db,
        JWTSecret:     "your-secret-key",
        TokenDuration: 7 * 24 * time.Hour, // 7天
    })
    
    // 注册用户（方式1：密码注册）
    u, token, err := userService.Register(&user.RegisterRequest{
        Phone:    "13800138000",
        Password: "123456",
    })
    
    // 注册用户（方式2：验证码注册）
    code, _ := userService.SendVerificationCode(&user.SendCodeRequest{
        Phone: "13900139000",
        Type:  user.CodeTypeRegister,
    })
    u, token, err = userService.Register(&user.RegisterRequest{
        Phone: "13900139000",
        Code:  code,
    })
    
    // 登录（方式1：密码登录，支持手机号或用户名）
    u, token, err = userService.Login(&user.LoginRequest{
        Account:  "13800138000", // 或使用用户名
        Password: "123456",
    })
    
    // 登录（方式2：验证码登录，仅支持手机号）
    code, _ = userService.SendVerificationCode(&user.SendCodeRequest{
        Phone: "13800138000",
        Type:  user.CodeTypeLogin,
    })
    u, token, err = userService.Login(&user.LoginRequest{
        Account: "13800138000",
        Code:    code,
    })
    
    // 验证Token
    claims, err := userService.ValidateToken(token)
    userID := claims.UserID
    
    // 获取用户信息
    u, err = userService.GetUserByID(userID)
    
    // 更新用户信息
    nickname := "新昵称"
    u, err = userService.UpdateProfile(userID, &user.UpdateProfileRequest{
        Nickname: &nickname,
    })
}
```

## API 接口

### 认证相关

#### 注册
```go
Register(req *RegisterRequest) (*User, string, error)

// RegisterRequest 结构
type RegisterRequest struct {
    Phone    string `json:"phone"`
    Password string `json:"password,omitempty"` // 密码注册时使用
    Code     string `json:"code,omitempty"`     // 验证码注册时使用
}
```

**注意**：密码和验证码至少需要提供一个
- 使用密码注册时，会自动生成 `user_` 开头的随机昵称
- 使用验证码注册时，也会自动生成随机昵称和密码

#### 密码登录
```go
Login(req *LoginRequest) (*User, string, error)

// LoginRequest 结构
type LoginRequest struct {
    Account  string `json:"account"`            // 账号：手机号或用户名
    Password string `json:"password,omitempty"` // 密码登录时使用
    Code     string `json:"code,omitempty"`     // 验证码登录时使用（仅手机号）
}
```

**支持三种登录方式**：
1. 手机号 + 密码
2. 用户名 + 密码
3. 手机号 + 验证码

#### 验证码登录
```go
LoginWithCode(phone, code string) (*User, string, error)
```

#### 修改密码
```go
ChangePassword(userID int64, req *ChangePasswordRequest) error
```

#### 重置密码
```go
ResetPassword(req *ResetPasswordRequest) error
```

### 验证码相关

#### 发送验证码
```go
SendVerificationCode(req *SendCodeRequest) (string, error)

// 验证码类型常量
const (
    CodeTypeRegister      = 1 // 注册
    CodeTypeLogin         = 2 // 登录
    CodeTypeResetPassword = 3 // 重置密码
)
```

#### 验证验证码
```go
VerifyCode(req *VerifyCodeRequest) error
```

### 用户信息相关

#### 获取用户信息
```go
GetUserByID(id int64) (*User, error)
GetUserProfile(id int64) (*UserProfile, error)
```

#### 更新用户信息
```go
UpdateProfile(userID int64, req *UpdateProfileRequest) (*User, error)
```

**支持更新的字段**：
- 昵称（nickname）
- 头像（avatar）
- 邮箱（email）
- 性别（gender）
- 生日（birthday）
- 个性签名（signature）

### JWT 相关

#### 验证Token
```go
ValidateToken(token string) (*JWTClaims, error)
```

#### 刷新Token
```go
RefreshToken(token string) (string, error)
```

## 数据模型

### User 用户
```go
type User struct {
    ID           int64
    Username     string  // 自动生成：u + 手机号
    Phone        string
    Nickname     string  // 自动生成：user_ + 随机数
    Avatar       string
    Email        string
    Gender       int     // 0-未知，1-男，2-女
    Birthday     *string // YYYY-MM-DD
    Signature    string
    Status       int     // 0-禁用，1-正常
    CreatedAt    int64
    UpdatedAt    int64
}
```

## 运行示例

```bash
# 进入示例目录
cd example

# 修改数据库连接（main.go 中的 dbDSN）
# 默认: root:yyy003014@tcp(localhost:3306)/user_test

# 运行
go run main.go

# 访问测试页面
http://localhost:8080
```

## 注意事项

1. **JWT Secret**: 生产环境必须使用强密钥
2. **密码强度**: 建议在应用层增加密码复杂度验证
3. **验证码发送**: `SendVerificationCode` 返回验证码供测试，生产环境需要集成短信服务
4. **数据库**: 使用 MySQL，时间戳为毫秒
5. **随机昵称**: 注册时自动生成 `user_` 开头的随机昵称，用户可以后续通过 `UpdateProfile` 修改
