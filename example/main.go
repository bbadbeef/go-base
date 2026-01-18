package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/bbadbeef/go-base/im"
	"github.com/bbadbeef/go-base/user"
)

var (
	httpPort = flag.Int("port", 8080, "HTTP端口")
	grpcPort = flag.Int("grpc", 50051, "gRPC端口")
	dbDSN    = flag.String("db", "root:yyy003014@tcp(localhost:3306)/im_user_test?parseTime=true", "数据库连接串")
	serverID = flag.String("id", "server-1", "服务器ID")
)

var (
	userService user.Service
	imService   im.IMService
)

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("启动集成服务器: %s", *serverID)

	// 连接数据库 (使用 GORM)
	db, err := gorm.Open(mysql.Open(*dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}
	log.Println("数据库连接成功")

	// 创建用户服务
	userService, err = user.NewService(&user.Config{
		DB:            db,
		JWTSecret:     "your-secret-key-change-in-production",
		TokenDuration: 7 * 24 * time.Hour,
	})
	if err != nil {
		log.Fatal("创建用户服务失败:", err)
	}
	log.Println("用户服务初始化成功")

	// 创建 IM 服务
	grpcAddr := fmt.Sprintf("0.0.0.0:%d", *grpcPort)
	imService = im.NewBuilder().
		WithServerID(*serverID).
		WithGRPCAddr(grpcAddr).
		WithDB(db).
		WithAuthFunc(validateToken). // 使用 JWT Token 认证
		WithCacheTTL(30).
		WithHeartbeatInterval(15).
		MustBuild()

	// 设置 IM 回调
	setupIMCallbacks()
	log.Println("IM 服务初始化成功")

	// 启动 IM 服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := imService.Start(ctx); err != nil {
			log.Printf("IM 服务错误: %v", err)
		}
	}()

	// 启动 HTTP 服务
	mux := http.NewServeMux()
	setupRoutes(mux)

	httpAddr := fmt.Sprintf(":%d", *httpPort)
	server := &http.Server{
		Addr:    httpAddr,
		Handler: enableCORS(mux),
	}

	go func() {
		log.Printf("HTTP 服务启动在 %s", httpAddr)
		log.Printf("访问测试页面: http://localhost%s", httpAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
	cancel()
	imService.Stop()
	server.Close()
	log.Println("服务器已关闭")
}

// validateToken 验证 Token 并返回 userID
func validateToken(token string) (int64, error) {
	claims, err := userService.ValidateToken(token)
	if err != nil {
		return 0, fmt.Errorf("invalid token: %w", err)
	}
	return claims.UserID, nil
}

// setupIMCallbacks 设置 IM 回调
func setupIMCallbacks() {
	imService.OnMessage(func(msg *im.Message) {
		log.Printf("[消息] %d -> %d: %s", msg.FromUserID, msg.ToUserID, msg.Content)
	})

	imService.OnUserOnline(func(userID int64) {
		log.Printf("[上线] 用户 %d", userID)
	})

	imService.OnUserOffline(func(userID int64) {
		log.Printf("[下线] 用户 %d", userID)
	})
}

// setupRoutes 设置路由
func setupRoutes(mux *http.ServeMux) {
	// 用户认证相关
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/code/send", handleSendCode)

	// 用户信息相关（需要认证）
	mux.HandleFunc("/api/user/profile", authMiddleware(handleGetProfile))
	mux.HandleFunc("/api/user/update", authMiddleware(handleUpdateProfile))

	// IM 相关（需要认证）
	mux.HandleFunc("/ws", imService.WebSocketHandler()) // WebSocket 连接
	mux.HandleFunc("/api/sessions", authMiddleware(handleGetSessions))
	mux.HandleFunc("/api/messages", authMiddleware(handleGetMessages))
	mux.HandleFunc("/api/send", authMiddleware(handleSendMessage))
	mux.HandleFunc("/api/online", handleCheckOnline)

	// 测试页面
	mux.HandleFunc("/", handleTestPage)
}

// ==================== 用户相关 API ====================

// 注册
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req user.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, token, err := userService.Register(&req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"user":  u,
			"token": token,
		},
	})
}

// 登录
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req user.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, token, err := userService.Login(&req)
	if err != nil {
		httpError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"user":  u,
			"token": token,
		},
	})
}

// 发送验证码
func handleSendCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req user.SendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	code, err := userService.SendVerificationCode(&req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"message": "验证码已发送",
			"code":    code, // 仅测试环境返回
		},
	})
}

// 获取用户信息
func handleGetProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	u, err := userService.GetUserByID(userID)
	if err != nil {
		httpError(w, err.Error(), http.StatusNotFound)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": u,
	})
}

// 更新用户信息
func handleUpdateProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req user.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := userService.UpdateProfile(userID, &req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": u,
	})
}

// ==================== IM 相关 API ====================

// 获取会话列表
func handleGetSessions(w http.ResponseWriter, r *http.Request, userID int64) {
	sessions, err := imService.GetSessions(r.Context(), userID)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": sessions,
	})
}

// 获取历史消息
func handleGetMessages(w http.ResponseWriter, r *http.Request, userID int64) {
	targetID, _ := strconv.ParseInt(r.URL.Query().Get("target_id"), 10, 64)
	sessionType, _ := strconv.Atoi(r.URL.Query().Get("session_type"))
	if sessionType == 0 {
		sessionType = im.SessionTypeSingle
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}

	messages, err := imService.GetMessages(r.Context(), &im.GetMessagesRequest{
		UserID:      userID,
		TargetID:    targetID,
		SessionType: sessionType,
		BeforeTime:  0,
		Limit:       limit,
	})
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": messages,
	})
}

// 发送消息
func handleSendMessage(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req im.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.FromUserID = userID // 使用认证的用户ID

	if req.MsgType == 0 {
		req.MsgType = im.MsgTypeText
	}

	err := imService.SendMessage(r.Context(), &req)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code":    200,
		"message": "success",
	})
}

// 检查用户是否在线
func handleCheckOnline(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if err != nil {
		httpError(w, "无效的 user_id", http.StatusBadRequest)
		return
	}

	online := imService.IsUserOnline(userID)
	jsonResponse(w, map[string]interface{}{
		"code":   200,
		"online": online,
	})
}

// ==================== 中间件 ====================

// authMiddleware 认证中间件
func authMiddleware(handler func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := getTokenFromRequest(r)
		if token == "" {
			httpError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := userService.ValidateToken(token)
		if err != nil {
			httpError(w, "invalid token", http.StatusUnauthorized)
			return
		}

		handler(w, r, claims.UserID)
	}
}

// getTokenFromRequest 从请求中获取Token
func getTokenFromRequest(r *http.Request) string {
	// 从Header中获取
	auth := r.Header.Get("Authorization")
	if auth != "" {
		parts := strings.Split(auth, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// 从Query参数中获取
	return r.URL.Query().Get("token")
}

// ==================== 工具函数 ====================

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func httpError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":  code,
		"error": message,
	})
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ==================== 测试页面 ====================

func handleTestPage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>IM 聊天系统</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: Arial, sans-serif; background: #f0f2f5; }
        
        .container { display: flex; height: 100vh; }
        
        /* 左侧栏 */
        .sidebar { width: 300px; background: white; border-right: 1px solid #e5e5e5; display: flex; flex-direction: column; }
        .sidebar-header { padding: 20px; background: #2196F3; color: white; }
        .sidebar-header h2 { margin-bottom: 10px; }
        .user-info { font-size: 14px; }
        
        /* 认证面板 */
        .auth-panel { padding: 20px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-size: 14px; color: #333; }
        .form-group input { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
        .btn { width: 100%; padding: 10px; background: #2196F3; color: white; border: none; border-radius: 4px; cursor: pointer; margin-bottom: 10px; }
        .btn:hover { background: #1976D2; }
        .btn-secondary { background: #666; }
        .btn-secondary:hover { background: #555; }
        
        /* 会话列表 */
        .session-list { flex: 1; overflow-y: auto; }
        .session-item { padding: 15px; border-bottom: 1px solid #f0f0f0; cursor: pointer; transition: background 0.2s; }
        .session-item:hover { background: #f5f5f5; }
        .session-item.active { background: #e3f2fd; }
        .session-name { font-weight: bold; margin-bottom: 5px; }
        .session-last-msg { font-size: 12px; color: #999; }
        
        /* 主聊天区域 */
        .main-content { flex: 1; display: flex; flex-direction: column; background: white; }
        .chat-header { padding: 15px 20px; border-bottom: 1px solid #e5e5e5; background: white; }
        .chat-header h3 { color: #333; }
        
        .chat-messages { flex: 1; padding: 20px; overflow-y: auto; background: #f5f5f5; }
        .message { margin-bottom: 20px; display: flex; }
        .message.sent { justify-content: flex-end; }
        .message.received { justify-content: flex-start; }
        
        .message-content { max-width: 60%; padding: 10px 15px; border-radius: 8px; word-wrap: break-word; }
        .message.sent .message-content { background: #2196F3; color: white; }
        .message.received .message-content { background: white; color: #333; border: 1px solid #e5e5e5; }
        
        .message-info { font-size: 11px; margin-top: 5px; opacity: 0.7; }
        
        .chat-input { padding: 20px; border-top: 1px solid #e5e5e5; background: white; }
        .input-box { display: flex; gap: 10px; }
        .input-box input { flex: 1; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        .input-box button { padding: 10px 30px; background: #2196F3; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .input-box button:hover { background: #1976D2; }
        
        /* 欢迎页面 */
        .welcome { flex: 1; display: flex; align-items: center; justify-content: center; color: #999; font-size: 18px; }
        
        .hidden { display: none !important; }
        
        .status-badge { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 5px; }
        .status-online { background: #4caf50; }
        .status-offline { background: #999; }
    </style>
</head>
<body>
    <div class="container">
        <!-- 左侧栏 -->
        <div class="sidebar">
            <div class="sidebar-header">
                <h2>IM 聊天系统</h2>
                <div class="user-info" id="userInfo">未登录</div>
            </div>
            
            <!-- 认证面板 -->
            <div class="auth-panel" id="authPanel">
                <div class="form-group">
                    <label>用户名/手机号:</label>
                    <input type="text" id="username" placeholder="testuser / 13800138000">
                </div>
                <div class="form-group">
                    <label>手机号:</label>
                    <input type="text" id="phone" placeholder="13800138000">
                </div>
                <div class="form-group">
                    <label>密码:</label>
                    <input type="password" id="password" placeholder="123456">
                </div>
                <button class="btn" onclick="register()">注册</button>
                <button class="btn btn-secondary" onclick="login()">登录</button>
            </div>
            
            <!-- 会话列表 -->
            <div class="session-list hidden" id="sessionList"></div>
            
            <button class="btn btn-secondary" onclick="logout()" id="logoutBtn" style="margin: 10px; display: none;">退出登录</button>
        </div>
        
        <!-- 主聊天区域 -->
        <div class="main-content">
            <div class="welcome" id="welcomeScreen">
                请先登录或注册
            </div>
            
            <div class="hidden" id="chatArea">
                <div class="chat-header">
                    <h3 id="chatTitle">选择一个会话开始聊天</h3>
                </div>
                <div class="chat-messages" id="chatMessages"></div>
                <div class="chat-input">
                    <div class="input-box">
                        <input type="text" id="messageInput" placeholder="输入消息..." onkeypress="handleKeyPress(event)">
                        <button onclick="sendMessage()">发送</button>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let token = '';
        let currentUser = null;
        let ws = null;
        let currentTargetUser = null;
        let sessions = [];

        // 注册
        async function register() {
            const username = document.getElementById('username').value.trim();
            const phone = document.getElementById('phone').value.trim();
            const password = document.getElementById('password').value.trim();

            if (!username || !phone || !password) {
                alert('请填写完整信息');
                return;
            }

            const result = await apiCall('/api/register', {
                username: username,
                phone: phone,
                password: password
            });

            if (result.code === 200) {
                token = result.data.token;
                currentUser = result.data.user;
                onLoginSuccess();
            } else {
                alert(result.error || '注册失败');
            }
        }

        // 登录
        async function login() {
            const phone = document.getElementById('phone').value.trim();
            const password = document.getElementById('password').value.trim();

            if (!phone || !password) {
                alert('请填写手机号和密码');
                return;
            }

            const result = await apiCall('/api/login', {
                phone: phone,
                password: password
            });

            if (result.code === 200) {
                token = result.data.token;
                currentUser = result.data.user;
                onLoginSuccess();
            } else {
                alert(result.error || '登录失败');
            }
        }

        // 登录成功处理
        function onLoginSuccess() {
            document.getElementById('authPanel').classList.add('hidden');
            document.getElementById('sessionList').classList.remove('hidden');
            document.getElementById('logoutBtn').style.display = 'block';
            document.getElementById('welcomeScreen').classList.add('hidden');
            document.getElementById('chatArea').classList.remove('hidden');
            document.getElementById('userInfo').innerHTML = 
                '<span class="status-badge status-online"></span>' + currentUser.nickname + ' (ID: ' + currentUser.id + ')';
            
            connectWebSocket();
            loadSessions();
            
            // 添加一个示例会话
            addSampleSession();
        }

        // 退出登录
        function logout() {
            if (ws) ws.close();
            token = '';
            currentUser = null;
            currentTargetUser = null;
            
            document.getElementById('authPanel').classList.remove('hidden');
            document.getElementById('sessionList').classList.add('hidden');
            document.getElementById('logoutBtn').style.display = 'none';
            document.getElementById('welcomeScreen').classList.remove('hidden');
            document.getElementById('chatArea').classList.add('hidden');
            document.getElementById('userInfo').textContent = '未登录';
            document.getElementById('sessionList').innerHTML = '';
            document.getElementById('chatMessages').innerHTML = '';
        }

        // 连接 WebSocket
        function connectWebSocket() {
            const wsUrl = 'ws://' + window.location.host + '/ws?token=' + token;
            ws = new WebSocket(wsUrl);

            ws.onopen = () => {
                console.log('WebSocket 已连接');
                startHeartbeat();
            };

            ws.onclose = () => {
                console.log('WebSocket 已断开');
            };

            ws.onerror = (error) => {
                console.error('WebSocket 错误:', error);
            };

            ws.onmessage = (event) => {
                const msg = JSON.parse(event.data);
                handleWebSocketMessage(msg);
            };
        }

        let heartbeatTimer = null;
        function startHeartbeat() {
            heartbeatTimer = setInterval(() => {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ type: 'ping', timestamp: Date.now() }));
                }
            }, 30000);
        }

        // 处理 WebSocket 消息
        function handleWebSocketMessage(msg) {
            console.log('收到消息:', msg);

            switch (msg.type) {
                case 'pong':
                    break;

                case 'chat_msg':
                    if (currentTargetUser && msg.data.from_user_id === currentTargetUser.id) {
                        displayMessage({
                            content: msg.data.content,
                            isSent: false,
                            time: msg.data.server_time
                        });
                    }
                    // 发送已送达回执
                    ws.send(JSON.stringify({
                        type: 'delivered_receipt',
                        msg_id: msg.msg_id,
                        data: { msg_id: msg.msg_id, type: 'delivered', time: Date.now() },
                        timestamp: Date.now()
                    }));
                    break;

                case 'ack':
                    console.log('消息已确认:', msg.msg_id);
                    break;
            }
        }

        // 加载会话列表
        async function loadSessions() {
            const result = await apiCall('/api/sessions', null, token);
            if (result.code === 200) {
                sessions = result.data || [];
                renderSessions();
            }
        }

        // 添加示例会话
        function addSampleSession() {
            const targetUserId = prompt('请输入要聊天的用户ID:');
            if (targetUserId) {
                selectUser(parseInt(targetUserId), 'User ' + targetUserId);
            }
        }

        // 渲染会话列表
        function renderSessions() {
            const list = document.getElementById('sessionList');
            list.innerHTML = '<div style="padding: 10px; text-align: center;"><button class="btn" onclick="addSampleSession()">+ 新建会话</button></div>';
            
            sessions.forEach(session => {
                const div = document.createElement('div');
                div.className = 'session-item';
                div.innerHTML = 
                    '<div class="session-name">用户 ' + session.target_id + '</div>' +
                    '<div class="session-last-msg">' + (session.last_message || '暂无消息') + '</div>';
                div.onclick = () => selectUser(session.target_id, 'User ' + session.target_id);
                list.appendChild(div);
            });
        }

        // 选择用户
        async function selectUser(userId, nickname) {
            currentTargetUser = { id: userId, nickname: nickname };
            document.getElementById('chatTitle').textContent = nickname;
            document.getElementById('chatMessages').innerHTML = '';
            
            // 加载历史消息
            const result = await apiCall('/api/messages?target_id=' + userId + '&limit=50', null, token);
            if (result.code === 200) {
                const messages = result.data || [];
                messages.reverse().forEach(msg => {
                    displayMessage({
                        content: msg.content,
                        isSent: msg.from_user_id === currentUser.id,
                        time: msg.server_time
                    });
                });
            }
        }

        // 发送消息
        function sendMessage() {
            if (!currentTargetUser) {
                alert('请先选择聊天对象');
                return;
            }

            const input = document.getElementById('messageInput');
            const content = input.value.trim();
            if (!content) return;

            const msgId = generateUUID();
            const msg = {
                type: 'chat_msg',
                msg_id: msgId,
                data: {
                    msg_id: msgId,
                    to_user_id: currentTargetUser.id,
                    content: content,
                    msg_type: 1,
                    client_time: Date.now()
                },
                timestamp: Date.now()
            };

            ws.send(JSON.stringify(msg));
            input.value = '';

            displayMessage({
                content: content,
                isSent: true,
                time: Date.now()
            });
        }

        // 显示消息
        function displayMessage(msg) {
            const messagesDiv = document.getElementById('chatMessages');
            const msgDiv = document.createElement('div');
            msgDiv.className = 'message ' + (msg.isSent ? 'sent' : 'received');

            const contentDiv = document.createElement('div');
            contentDiv.className = 'message-content';
            contentDiv.textContent = msg.content;

            const infoDiv = document.createElement('div');
            infoDiv.className = 'message-info';
            infoDiv.textContent = new Date(msg.time).toLocaleTimeString();

            contentDiv.appendChild(infoDiv);
            msgDiv.appendChild(contentDiv);
            messagesDiv.appendChild(msgDiv);
            messagesDiv.scrollTop = messagesDiv.scrollHeight;
        }

        // API 调用
        async function apiCall(url, data, authToken) {
            try {
                const options = {
                    method: data ? 'POST' : 'GET',
                    headers: { 'Content-Type': 'application/json' }
                };

                if (data) {
                    options.body = JSON.stringify(data);
                }

                if (authToken) {
                    options.headers['Authorization'] = 'Bearer ' + authToken;
                }

                const response = await fetch(url, options);
                return await response.json();
            } catch (error) {
                return { error: error.message };
            }
        }

        function generateUUID() {
            return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
                const r = Math.random() * 16 | 0;
                const v = c === 'x' ? r : (r & 0x3 | 0x8);
                return v.toString(16);
            });
        }

        function handleKeyPress(event) {
            if (event.key === 'Enter') {
                sendMessage();
            }
        }
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
