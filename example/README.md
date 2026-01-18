# IM + User 集成示例

这是一个完整的示例应用，集成了用户管理和即时通讯功能，提供注册、登录、聊天等完整功能。

## 功能特性

- ✅ 用户注册（支持密码或验证码注册）
- ✅ 用户登录（支持密码或验证码登录）
- ✅ 自动生成随机昵称（user_开头）
- ✅ JWT Token 认证
- ✅ 实时聊天（WebSocket）
- ✅ 历史消息查询
- ✅ 用户在线状态
- ✅ 离线消息推送
- ✅ 文件上传（图片、视频、语音、文件）
- ✅ 头像上传和更新
- ✅ 漂亮的 Web UI 界面

## 快速开始

### 1. 准备数据库

```bash
# 创建数据库（表会自动创建）
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS im_user_test DEFAULT CHARACTER SET utf8mb4;"
```

### 2. 配置数据库连接

如果需要修改数据库连接，编辑 `main.go`：

```go
dbDSN = flag.String("db", "root:yyy003014@tcp(localhost:3306)/im_user_test?parseTime=true", "数据库连接串")
```

### 3. 运行服务

```bash
# 直接运行（会自动创建表）
go run main.go

# 或指定端口
go run main.go -port 8080 -grpc 50051
```

### 4. 访问测试页面

打开浏览器访问：http://localhost:8080

## 使用说明

### 注册账号

支持两种注册方式：

**方式1：手机号 + 密码**
1. 填写手机号（如：13800138000）
2. 填写密码（如：123456）
3. 点击"注册（密码或验证码）"按钮
4. 系统自动生成 user_ 开头的随机昵称

**方式2：手机号 + 验证码**
1. 填写手机号
2. 点击"**注册码**"按钮获取注册验证码
3. 填写收到的验证码
4. 点击"注册（密码或验证码）"按钮

**注意**：注册和登录使用不同的验证码，请点击对应的按钮。

### 登录

支持三种登录方式：

**方式1：手机号 + 密码**
1. 填写手机号（如：13800138000）
2. 填写密码
3. 点击"登录（密码或验证码）"按钮

**方式2：用户名 + 密码**
1. 填写用户名（格式：u + 手机号，如：u13800138000）
2. 填写密码
3. 点击"登录（密码或验证码）"按钮

**方式3：手机号 + 验证码**
1. 填写手机号
2. 点击"**登录码**"按钮获取登录验证码（注意不是注册码）
3. 填写验证码
4. 点击"登录（密码或验证码）"按钮

**重要提示**：
- 注册时请点击"**注册码**"按钮获取注册验证码
- 登录时请点击"**登录码**"按钮获取登录验证码
- 两种验证码不能混用

### 修改头像

1. 登录后，点击左上角的头像
2. 选择图片文件（支持 jpg、png 等格式，最大 5MB）
3. 上传成功后自动更新显示

### 开始聊天

1. 点击"+ 新建会话"按钮
2. 输入对方的用户ID（需要另一个已注册的用户）
3. 在输入框输入消息，点击"发送"或按回车键
4. 实时收发消息

### 发送多媒体消息

- 📷 **图片**：点击"📷 图片"按钮选择图片文件
- 🎬 **视频**：点击"🎬 视频"按钮选择视频文件
- 🎤 **语音**：点击"🎤 语音"按钮选择音频文件
- 📎 **文件**：点击"📎 文件"按钮选择任意文件

### 多人聊天测试

在不同的浏览器窗口或隐身模式下：
1. 使用不同手机号注册不同的账号（如 13800138000、13900139000）
2. 记下各自的用户ID（登录后显示在左上角）
3. 在各自的界面中添加对方为会话对象
4. 开始聊天测试

## 架构说明

```
example/
├── main.go                 # 主程序
├── go.mod                  # 依赖管理
├── sql/
│   └── init_db.sh         # 数据库初始化脚本
└── README.md
```

### 核心流程

1. **用户认证**：User 模块处理注册、登录，生成 JWT Token
2. **WebSocket 连接**：客户端使用 Token 连接 WebSocket
3. **Token 验证**：IM 模块调用 User 模块验证 Token，获取用户ID
4. **消息收发**：通过 WebSocket 实时收发消息
5. **离线消息**：用户上线时自动推送离线消息
6. **文件管理**：Storage 模块处理文件上传和下载

### 关键代码

#### Token 验证集成
```go
// 创建 IM 服务时，注入 Token 验证函数
imService = im.NewBuilder().
    WithAuthFunc(func(token string) (int64, error) {
        claims, err := userService.ValidateToken(token)
        if err != nil {
            return 0, err
        }
        return claims.UserID, nil
    }).
    MustBuild()
```

#### API 认证中间件
```go
func authMiddleware(handler func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        token := getTokenFromRequest(r)
        claims, err := userService.ValidateToken(token)
        if err != nil {
            httpError(w, "invalid token", http.StatusUnauthorized)
            return
        }
        handler(w, r, claims.UserID)
    }
}
```

## API 接口

### 用户相关

- `POST /api/register` - 用户注册
  ```json
  {
    "phone": "13800138000",
    "password": "123456"  // 或使用 "code": "123456"
  }
  ```

- `POST /api/login` - 用户登录
  ```json
  {
    "account": "13800138000",  // 或用户名
    "password": "123456"       // 或使用 "code": "123456"
  }
  ```

- `POST /api/code/send` - 发送验证码
  ```json
  {
    "phone": "13800138000",
    "type": 1  // 1-注册，2-登录，3-重置密码
  }
  ```

- `GET /api/user/profile` - 获取用户信息（需认证）
- `GET /api/user/info?user_id=xxx` - 获取其他用户信息（需认证）
- `POST /api/user/update` - 更新用户信息（需认证）

### 文件上传相关

- `POST /api/upload/image` - 上传图片（需认证）
- `POST /api/upload/video` - 上传视频（需认证）
- `POST /api/upload/voice` - 上传语音（需认证）
- `POST /api/upload/file` - 上传文件（需认证）
- `POST /api/upload/avatar` - 上传头像（需认证）
- `GET /api/files/{file_id}` - 下载文件

### IM 相关

- `GET /ws?token=xxx` - WebSocket 连接（需Token）
- `GET /api/sessions` - 获取会话列表（需认证）
- `GET /api/messages?target_id=xxx` - 获取历史消息（需认证）
- `POST /api/send` - 发送消息（需认证）
- `GET /api/online?user_id=xxx` - 检查用户在线状态

## 数据库表

使用 `im_user_test` 数据库，包含以下表：

### User 模块
- `user_users` - 用户信息表
- `user_verification_codes` - 验证码表

### IM 模块
- `im_messages` - 消息表
- `im_sessions` - 会话表
- `im_groups` - 群组表
- `im_group_members` - 群成员表

### Storage 模块
- `storage_files` - 文件信息表

## 命令行参数

```bash
go run main.go [options]

Options:
  -port int      HTTP端口 (default 8080)
  -grpc int      gRPC端口 (default 50051)
  -db string     数据库连接串
  -id string     服务器ID (default "server-1")
```

## 故障排查

### 数据库连接失败
- 检查 MySQL 是否运行
- 检查数据库连接参数
- 确保数据库 `im_user_test` 已创建

### 注册失败
- 检查手机号格式（必须是11位，以1开头）
- 确保密码或验证码二选一填写
- 验证码5分钟内有效

### 登录失败
- 检查账号是否已注册
- 密码登录时，账号支持手机号或用户名
- 验证码登录时，账号只能是手机号

### WebSocket 连接失败
- 检查 Token 是否有效
- 查看浏览器控制台错误信息
- 检查服务器日志

### 消息发送失败
- 确保对方用户ID正确
- 检查对方是否在线
- 查看服务器日志中的错误信息

### 文件上传失败
- 检查文件大小（最大10MB）
- 确保文件类型正确
- 检查存储目录权限

## 生产部署建议

1. **修改 JWT Secret**：使用强密钥替换默认密钥
2. **使用 HTTPS**：生产环境使用 wss:// 而非 ws://
3. **配置反向代理**：使用 Nginx 处理 WebSocket 和 HTTP
4. **日志管理**：配置日志文件和日志级别
5. **数据库优化**：配置连接池、索引优化
6. **文件存储**：使用对象存储（如 S3、OSS）替代本地存储
7. **验证码服务**：接入真实的短信服务提供商
8. **监控告警**：添加性能监控和错误告警

## 扩展功能

可以基于此示例继续扩展：

- ✅ 群聊功能
- ✅ 文件传输
- ✅ 消息已读回执
- ✅ 用户头像上传
- 好友关系管理
- 消息搜索
- 表情包支持
- 语音/视频通话
- 消息撤回
- @提醒功能

## License

MIT
