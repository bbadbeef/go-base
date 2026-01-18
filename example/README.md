# IM + User 集成示例

这是一个完整的示例应用，集成了用户管理和即时通讯功能，提供注册、登录、聊天等完整功能。

## 功能特性

- ✅ 用户注册
- ✅ 用户登录（密码认证）
- ✅ JWT Token 认证
- ✅ 实时聊天（WebSocket）
- ✅ 历史消息查询
- ✅ 用户在线状态
- ✅ 离线消息推送
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

1. 在左侧填写用户名、手机号、密码
2. 点击"注册"按钮
3. 注册成功后自动登录并连接 WebSocket

### 登录

1. 填写手机号和密码
2. 点击"登录"按钮
3. 登录成功后进入聊天界面

### 开始聊天

1. 点击"+ 新建会话"按钮
2. 输入对方的用户ID（需要另一个已注册的用户）
3. 在输入框输入消息，点击"发送"或按回车键
4. 实时收发消息

### 多人聊天测试

在不同的浏览器窗口或隐身模式下：
1. 注册不同的账号（如 user1、user2）
2. 记下各自的用户ID
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
- `POST /api/login` - 用户登录
- `GET /api/user/profile` - 获取用户信息（需认证）
- `POST /api/user/update` - 更新用户信息（需认证）

### IM 相关

- `GET /ws?token=xxx` - WebSocket 连接（需Token）
- `GET /api/sessions` - 获取会话列表（需认证）
- `GET /api/messages?target_id=xxx` - 获取历史消息（需认证）
- `POST /api/send` - 发送消息（需认证）
- `GET /api/online?user_id=xxx` - 检查用户在线状态

## 数据库表

使用 `im_user_test` 数据库，包含以下表：

### User 模块
- `users` - 用户信息表
- `verification_codes` - 验证码表

### IM 模块
- `im_messages` - 消息表
- `im_sessions` - 会话表
- `im_groups` - 群组表
- `im_group_members` - 群成员表

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

### WebSocket 连接失败
- 检查 Token 是否有效
- 查看浏览器控制台错误信息
- 检查服务器日志

### 消息发送失败
- 确保对方用户ID正确
- 检查对方是否在线
- 查看服务器日志中的错误信息

## 生产部署建议

1. **修改 JWT Secret**：使用强密钥替换默认密钥
2. **使用 HTTPS**：生产环境使用 wss:// 而非 ws://
3. **配置反向代理**：使用 Nginx 处理 WebSocket 和 HTTP
4. **日志管理**：配置日志文件和日志级别
5. **数据库优化**：配置连接池、索引优化
6. **监控告警**：添加性能监控和错误告警

## 扩展功能

可以基于此示例继续扩展：

- 群聊功能
- 文件传输
- 消息已读回执
- 用户头像上传
- 好友关系管理
- 消息搜索
- 表情包支持

## License

MIT
