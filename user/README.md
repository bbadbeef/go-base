# User 用户管理模块

提供完整的用户管理功能，包括注册、登录、验证码认证、用户信息管理等。

## 功能特性

- ✅ 用户注册（支持验证码可选）
- ✅ 密码登录
- ✅ 验证码登录
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
    
    // 注册用户
    u, token, err := userService.Register(&user.RegisterRequest{
        Username: "testuser",
        Phone:    "13800138000",
        Password: "123456",
    })
    
    // 登录
    u, token, err = userService.Login(&user.LoginRequest{
        Phone:    "13800138000",
        Password: "123456",
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
```

#### 密码登录
```go
Login(req *LoginRequest) (*User, string, error)
```

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
    Username     string
    Phone        string
    Nickname     string
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
http://localhost:8081
```

## 注意事项

1. **JWT Secret**: 生产环境必须使用强密钥
2. **密码强度**: 建议在应用层增加密码复杂度验证
3. **验证码发送**: `SendVerificationCode` 返回验证码供测试，生产环境需要集成短信服务
4. **数据库**: 使用 MySQL，时间戳为毫秒
