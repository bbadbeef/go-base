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
	"github.com/bbadbeef/go-base/storage"
	"github.com/bbadbeef/go-base/user"
)

var (
	httpPort = flag.Int("port", 8080, "HTTP端口")
	grpcPort = flag.Int("grpc", 50051, "gRPC端口")
	dbDSN    = flag.String("db", "root:yyy003014@tcp(localhost:3306)/im_user_test?parseTime=true", "数据库连接串")
	serverID = flag.String("id", "server-1", "服务器ID")
)

var (
	userService    user.Service
	imService      im.IMService
	storageService storage.Storage
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

	// 创建存储服务
	storageService, err = storage.NewStorage(&storage.Config{
		DB:      db,
		BaseURL: fmt.Sprintf("http://localhost:%d", *httpPort),
	})
	if err != nil {
		log.Fatal("创建存储服务失败:", err)
	}
	log.Println("存储服务初始化成功")

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
	mux.HandleFunc("/api/user/info", authMiddleware(handleGetUserInfo)) // 获取其他用户信息
	mux.HandleFunc("/api/user/update", authMiddleware(handleUpdateProfile))

	// 文件上传相关（需要认证）
	mux.HandleFunc("/api/upload/image", authMiddleware(handleUploadImage))
	mux.HandleFunc("/api/upload/video", authMiddleware(handleUploadVideo))
	mux.HandleFunc("/api/upload/voice", authMiddleware(handleUploadVoice))
	mux.HandleFunc("/api/upload/file", authMiddleware(handleUploadFile))
	mux.HandleFunc("/api/upload/avatar", authMiddleware(handleUploadAvatar))
	mux.HandleFunc("/api/files/", handleDownloadFile) // 文件下载（无需认证）

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

// 获取其他用户的公开信息
func handleGetUserInfo(w http.ResponseWriter, r *http.Request, _ int64) {
	// 从查询参数获取目标用户ID
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		httpError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	targetUserID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		httpError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	u, err := userService.GetUserByID(targetUserID)
	if err != nil {
		httpError(w, err.Error(), http.StatusNotFound)
		return
	}

	// 只返回公开信息
	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"id":       u.ID,
			"username": u.Username,
			"nickname": u.Nickname,
			"avatar":   u.Avatar,
			"gender":   u.Gender,
			"signature": u.Signature,
		},
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

// ==================== 文件上传相关 API ====================

// 上传图片
func handleUploadImage(w http.ResponseWriter, r *http.Request, userID int64) {
	handleUploadFile0(w, r, userID, storage.FileTypeImage)
}

// 上传视频
func handleUploadVideo(w http.ResponseWriter, r *http.Request, userID int64) {
	handleUploadFile0(w, r, userID, storage.FileTypeVideo)
}

// 上传语音
func handleUploadVoice(w http.ResponseWriter, r *http.Request, userID int64) {
	handleUploadFile0(w, r, userID, storage.FileTypeVoice)
}

// 上传文件
func handleUploadFile(w http.ResponseWriter, r *http.Request, userID int64) {
	handleUploadFile0(w, r, userID, storage.FileTypeFile)
}

// 上传头像
func handleUploadAvatar(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析文件
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		httpError(w, "解析文件失败", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpError(w, "获取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 上传文件
	fileInfo, err := storageService.Upload(&storage.UploadRequest{
		File:     file,
		Header:   header,
		UserID:   userID,
		FileType: storage.FileTypeImage,
	})
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 更新用户头像
	_, err = userService.UpdateProfile(userID, &user.UpdateProfileRequest{
		Avatar: &fileInfo.URL,
	})
	if err != nil {
		httpError(w, "更新用户头像失败", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": fileInfo,
	})
}

// 通用文件上传处理
func handleUploadFile0(w http.ResponseWriter, r *http.Request, userID int64, fileType string) {
	if r.Method != http.MethodPost {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析文件
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		httpError(w, "解析文件失败", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpError(w, "获取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 上传文件
	fileInfo, err := storageService.Upload(&storage.UploadRequest{
		File:     file,
		Header:   header,
		UserID:   userID,
		FileType: fileType,
	})
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"code": 200,
		"data": fileInfo,
	})
}

// 下载文件
func handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	// 从 URL 中提取 file_id: /api/files/{file_id}
	path := r.URL.Path
	fileID := strings.TrimPrefix(path, "/api/files/")
	if fileID == "" {
		httpError(w, "文件ID不能为空", http.StatusBadRequest)
		return
	}

	// 下载文件
	data, fileInfo, err := storageService.Download(fileID)
	if err != nil {
		httpError(w, err.Error(), http.StatusNotFound)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", fileInfo.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", fileInfo.FileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.FileSize))
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 缓存1年

	// 写入文件数据
	w.Write(data)
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
        .user-info { font-size: 14px; display: flex; align-items: center; gap: 10px; }
        .user-avatar { width: 40px; height: 40px; border-radius: 50%; object-fit: cover; border: 2px solid white; cursor: pointer; }
        .user-avatar.default { background: #fff; color: #2196F3; display: flex; align-items: center; justify-content: center; font-size: 20px; font-weight: bold; }
        .user-details { flex: 1; }
        .user-name { font-weight: bold; margin-bottom: 3px; }
        .user-id { font-size: 12px; opacity: 0.9; }
        
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
        .session-item { padding: 15px; border-bottom: 1px solid #f0f0f0; cursor: pointer; transition: background 0.2s; display: flex; align-items: center; gap: 12px; }
        .session-item:hover { background: #f5f5f5; }
        .session-item.active { background: #e3f2fd; }
        .session-avatar { width: 48px; height: 48px; border-radius: 50%; object-fit: cover; flex-shrink: 0; }
        .session-avatar.default { background: #ccc; color: white; display: flex; align-items: center; justify-content: center; font-size: 20px; font-weight: bold; }
        .session-info { flex: 1; min-width: 0; }
        .session-name { font-weight: bold; margin-bottom: 5px; }
        .session-last-msg { font-size: 12px; color: #999; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
        
        /* 主聊天区域 */
        .main-content { flex: 1; display: flex; flex-direction: column; background: white; }
        .chat-header { padding: 15px 20px; border-bottom: 1px solid #e5e5e5; background: white; }
        .chat-header h3 { color: #333; }
        
        .chat-messages { flex: 1; padding: 20px; overflow-y: auto; background: #f5f5f5; }
        .message { margin-bottom: 20px; display: flex; align-items: flex-start; gap: 10px; }
        .message.sent { justify-content: flex-end; }
        .message.received { justify-content: flex-start; }
        
        .message-avatar { width: 36px; height: 36px; border-radius: 50%; object-fit: cover; flex-shrink: 0; }
        .message-avatar.default { background: #ccc; color: white; display: flex; align-items: center; justify-content: center; font-size: 16px; font-weight: bold; }
        .message.sent .message-avatar { order: 2; }
        
        .message-content { max-width: 60%; padding: 10px 15px; border-radius: 8px; word-wrap: break-word; }
        .message.sent .message-content { background: #2196F3; color: white; }
        .message.received .message-content { background: white; color: #333; border: 1px solid #e5e5e5; }
        
        .message-info { font-size: 11px; margin-top: 5px; opacity: 0.7; }
        
        .chat-input { padding: 20px; border-top: 1px solid #e5e5e5; background: white; }
        .input-toolbar { display: flex; gap: 8px; margin-bottom: 10px; }
        .toolbar-btn { padding: 8px 12px; background: #f5f5f5; border: 1px solid #ddd; border-radius: 4px; cursor: pointer; font-size: 14px; }
        .toolbar-btn:hover { background: #e0e0e0; }
        .input-box { display: flex; gap: 10px; }
        .input-box input { flex: 1; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        .input-box button { padding: 10px 30px; background: #2196F3; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .input-box button:hover { background: #1976D2; }
        .file-input { display: none; }
        
        /* 多媒体消息样式 */
        .message-image { max-width: 300px; border-radius: 8px; cursor: pointer; }
        .message-video { max-width: 400px; border-radius: 8px; }
        .message-voice { display: flex; align-items: center; gap: 10px; }
        .message-file { display: flex; align-items: center; gap: 10px; padding: 10px; background: #f5f5f5; border-radius: 4px; }
        .file-icon { font-size: 24px; }
        .uploading { opacity: 0.6; position: relative; }
        .uploading::after { content: '上传中...'; position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); background: rgba(0,0,0,0.7); color: white; padding: 5px 10px; border-radius: 4px; font-size: 12px; }
        
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
                    <label>手机号/用户名:</label>
                    <input type="text" id="phone" placeholder="13800138000 或 u13800138000">
                </div>
                <div class="form-group">
                    <label>密码（与验证码二选一）:</label>
                    <input type="password" id="password" placeholder="输入密码">
                </div>
                <div class="form-group">
                    <label>验证码（与密码二选一）:</label>
                    <div style="display: flex; gap: 5px;">
                        <input type="text" id="code" placeholder="输入验证码" style="flex: 1;">
                        <button class="btn" onclick="sendCodeForRegister()" style="width: auto; padding: 8px 12px; margin: 0; font-size: 12px;">注册码</button>
                        <button class="btn" onclick="sendCodeForLogin()" style="width: auto; padding: 8px 12px; margin: 0; font-size: 12px; background: #4CAF50;">登录码</button>
                    </div>
                </div>
                <button class="btn" onclick="register()">注册（密码或验证码）</button>
                <button class="btn btn-secondary" onclick="login()">登录（密码或验证码）</button>
                <div style="margin-top: 10px; font-size: 12px; color: #666; text-align: center;">
                    提示：注册需手机号，登录支持手机号或用户名<br>
                    <span style="color: #2196F3;">注册码</span>用于注册，<span style="color: #4CAF50;">登录码</span>用于登录
                </div>
            </div>
            
            <!-- 会话列表 -->
            <div class="session-list hidden" id="sessionList"></div>
            
            <button class="btn btn-secondary" onclick="logout()" id="logoutBtn" style="margin: 10px; display: none;">退出登录</button>
            
            <!-- 隐藏的头像上传input -->
            <input type="file" id="avatarInput" accept="image/*" style="display: none;" onchange="handleAvatarUpload(this)">
        </div>
        
        <!-- 主聊天区域 -->
        <div class="main-content">
            <div class="welcome" id="welcomeScreen">
                <div style="text-align: center; padding: 40px;">
                    <h2 style="color: #2196F3; margin-bottom: 30px;">🎉 欢迎使用 IM 聊天系统</h2>
                    <div style="text-align: left; max-width: 600px; margin: 0 auto; background: #f9f9f9; padding: 30px; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);">
                        <h3 style="margin-bottom: 20px; color: #333;">📝 功能说明</h3>
                        <div style="line-height: 2; color: #555;">
                            <p><strong>注册方式：</strong></p>
                            <ul style="margin: 10px 0 20px 20px;">
                                <li>方式1：手机号 + 密码</li>
                                <li>方式2：手机号 + 验证码（点击"<span style="color: #2196F3;">注册码</span>"获取）</li>
                                <li>注册后系统自动生成 user_ 开头的随机昵称</li>
                            </ul>
                            
                            <p><strong>登录方式：</strong></p>
                            <ul style="margin: 10px 0 20px 20px;">
                                <li>方式1：手机号 + 密码</li>
                                <li>方式2：用户名 + 密码（用户名格式：u13800138000）</li>
                                <li>方式3：手机号 + 验证码（点击"<span style="color: #4CAF50;">登录码</span>"获取）</li>
                            </ul>
                            
                            <p><strong>其他功能：</strong></p>
                            <ul style="margin: 10px 0 20px 20px;">
                                <li>点击头像可更换个人头像</li>
                                <li>支持发送文本、图片、视频、语音、文件</li>
                                <li>支持实时在线聊天</li>
                            </ul>
                            
                            <div style="margin-top: 30px; padding: 15px; background: #fff3cd; border-radius: 4px; border-left: 4px solid #ffc107;">
                                <strong>💡 重要提示：</strong><br>
                                • 验证码会直接显示在弹窗中（仅测试环境）<br>
                                • <span style="color: #2196F3; font-weight: bold;">注册码</span>用于注册，<span style="color: #4CAF50; font-weight: bold;">登录码</span>用于登录，不能混用<br>
                                • 验证码有效期为5分钟
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            
            <div class="hidden" id="chatArea">
                <div class="chat-header">
                    <h3 id="chatTitle">选择一个会话开始聊天</h3>
                </div>
                <div class="chat-messages" id="chatMessages"></div>
                <div class="chat-input">
                    <div class="input-toolbar">
                        <button class="toolbar-btn" onclick="document.getElementById('imageInput').click()">📷 图片</button>
                        <button class="toolbar-btn" onclick="document.getElementById('videoInput').click()">🎬 视频</button>
                        <button class="toolbar-btn" onclick="document.getElementById('voiceInput').click()">🎤 语音</button>
                        <button class="toolbar-btn" onclick="document.getElementById('fileInput').click()">📎 文件</button>
                    </div>
                    <div class="input-box">
                        <input type="text" id="messageInput" placeholder="输入消息..." onkeypress="handleKeyPress(event)">
                        <button onclick="sendMessage()">发送</button>
                    </div>
                    <input type="file" id="imageInput" class="file-input" accept="image/*" onchange="handleFileSelect(this, 'image')">
                    <input type="file" id="videoInput" class="file-input" accept="video/*" onchange="handleFileSelect(this, 'video')">
                    <input type="file" id="voiceInput" class="file-input" accept="audio/*" onchange="handleFileSelect(this, 'voice')">
                    <input type="file" id="fileInput" class="file-input" onchange="handleFileSelect(this, 'file')">
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
            const phone = document.getElementById('phone').value.trim();
            const password = document.getElementById('password').value.trim();
            const code = document.getElementById('code').value.trim();

            if (!phone) {
                alert('请填写手机号');
                return;
            }

            // 验证手机号格式
            if (!/^1[3-9]\d{9}$/.test(phone)) {
                alert('请输入正确的手机号格式');
                return;
            }

            if (!password && !code) {
                alert('请填写密码或验证码（二选一）');
                return;
            }

            if (password && code) {
                alert('密码和验证码只需填写一个即可');
                return;
            }

            const requestData = { phone: phone };
            if (password) {
                requestData.password = password;
            }
            if (code) {
                requestData.code = code;
            }

            const result = await apiCall('/api/register', requestData);

            if (result.code === 200) {
                token = result.data.token;
                currentUser = result.data.user;
                alert('注册成功！\n用户名：' + currentUser.username + '\n昵称：' + currentUser.nickname);
                onLoginSuccess();
            } else {
                alert('注册失败：' + (result.error || '未知错误'));
            }
        }

        // 登录
        async function login() {
            const account = document.getElementById('phone').value.trim();
            const password = document.getElementById('password').value.trim();
            const code = document.getElementById('code').value.trim();

            if (!account) {
                alert('请填写手机号或用户名');
                return;
            }

            if (!password && !code) {
                alert('请填写密码或验证码（二选一）');
                return;
            }

            if (password && code) {
                alert('密码和验证码只需填写一个即可');
                return;
            }

            // 如果使用验证码登录，必须是手机号
            if (code && !/^1[3-9]\d{9}$/.test(account)) {
                alert('验证码登录仅支持手机号');
                return;
            }

            const requestData = { account: account };
            if (password) {
                requestData.password = password;
            }
            if (code) {
                requestData.code = code;
            }

            const result = await apiCall('/api/login', requestData);

            if (result.code === 200) {
                token = result.data.token;
                currentUser = result.data.user;
                alert('登录成功！欢迎回来，' + currentUser.nickname);
                onLoginSuccess();
            } else {
                alert('登录失败：' + (result.error || '未知错误'));
            }
        }

        let registerCodeTimer = null;
        let loginCodeTimer = null;

        // 发送注册验证码
        async function sendCodeForRegister() {
            await sendCode(1, '注册', 'register');
        }

        // 发送登录验证码
        async function sendCodeForLogin() {
            await sendCode(2, '登录', 'login');
        }

        // 发送验证码（通用方法）
        async function sendCode(type, typeName, buttonType) {
            const phone = document.getElementById('phone').value.trim();
            
            if (!phone) {
                alert('请先填写手机号');
                return;
            }

            // 验证手机号格式
            if (!/^1[3-9]\d{9}$/.test(phone)) {
                alert('请输入正确的手机号格式');
                return;
            }

            // 获取对应的按钮
            const buttons = event.target.parentElement.querySelectorAll('button');
            const btn = event.target;
            const originalText = btn.textContent;
            
            btn.disabled = true;
            btn.textContent = '发送中...';

            const result = await apiCall('/api/code/send', {
                phone: phone,
                type: type  // 1-注册，2-登录
            });

            if (result.code === 200) {
                alert(typeName + '验证码已发送：' + result.data.code + '\n（测试环境直接显示，生产环境通过短信发送）');
                
                // 倒计时60秒
                let countdown = 60;
                const timer = setInterval(() => {
                    countdown--;
                    btn.textContent = countdown + '秒';
                    if (countdown <= 0) {
                        clearInterval(timer);
                        btn.disabled = false;
                        btn.textContent = originalText;
                        if (buttonType === 'register') {
                            registerCodeTimer = null;
                        } else {
                            loginCodeTimer = null;
                        }
                    }
                }, 1000);
                
                if (buttonType === 'register') {
                    registerCodeTimer = timer;
                } else {
                    loginCodeTimer = timer;
                }
            } else {
                alert(result.error || '发送验证码失败');
                btn.disabled = false;
                btn.textContent = originalText;
            }
        }

        // 清除倒计时
        function clearCodeTimers() {
            if (registerCodeTimer) {
                clearInterval(registerCodeTimer);
                registerCodeTimer = null;
            }
            if (loginCodeTimer) {
                clearInterval(loginCodeTimer);
                loginCodeTimer = null;
            }
        }

        // 登录成功处理
        function onLoginSuccess() {
            // 清空输入框
            document.getElementById('phone').value = '';
            document.getElementById('password').value = '';
            document.getElementById('code').value = '';
            
            // 清除验证码倒计时
            clearCodeTimers();
            
            // 重置验证码按钮状态
            const buttons = document.querySelectorAll('.auth-panel button');
            buttons.forEach(btn => {
                if (btn.textContent.includes('秒')) {
                    btn.disabled = false;
                    if (btn.onclick === sendCodeForRegister) {
                        btn.textContent = '注册码';
                    } else if (btn.onclick === sendCodeForLogin) {
                        btn.textContent = '登录码';
                    }
                }
            });
            
            document.getElementById('authPanel').classList.add('hidden');
            document.getElementById('sessionList').classList.remove('hidden');
            document.getElementById('logoutBtn').style.display = 'block';
            document.getElementById('welcomeScreen').classList.add('hidden');
            document.getElementById('chatArea').classList.remove('hidden');
            
            // 显示用户信息和头像
            updateUserInfo();
            
            connectWebSocket();
            loadSessions();
            
            // 添加一个示例会话
            addSampleSession();
        }

        // 更新用户信息显示
        function updateUserInfo() {
            let avatarHTML;
            if (currentUser.avatar) {
                avatarHTML = '<img src="' + currentUser.avatar + '" class="user-avatar" onclick="document.getElementById(\'avatarInput\').click()" title="点击更换头像">';
            } else {
                avatarHTML = '<div class="user-avatar default" onclick="document.getElementById(\'avatarInput\').click()" title="点击设置头像">' + currentUser.nickname.charAt(0).toUpperCase() + '</div>';
            }
            
            document.getElementById('userInfo').innerHTML = 
                avatarHTML +
                '<div class="user-details">' +
                    '<div class="user-name">' +
                        '<span class="status-badge status-online"></span>' + currentUser.nickname +
                    '</div>' +
                    '<div class="user-id">ID: ' + currentUser.id + '</div>' +
                '</div>';
        }

        // 处理头像上传
        async function handleAvatarUpload(input) {
            const file = input.files[0];
            if (!file) return;

            // 文件大小检查（5MB）
            if (file.size > 5 * 1024 * 1024) {
                alert('头像文件大小不能超过 5MB');
                input.value = '';
                return;
            }

            // 检查是否为图片
            if (!file.type.startsWith('image/')) {
                alert('请选择图片文件');
                input.value = '';
                return;
            }

            try {
                const formData = new FormData();
                formData.append('file', file);

                const response = await fetch('/api/upload/avatar', {
                    method: 'POST',
                    headers: {
                        'Authorization': 'Bearer ' + token
                    },
                    body: formData
                });

                const result = await response.json();

                if (result.code === 200) {
                    // 更新当前用户的头像
                    currentUser.avatar = result.data.url;
                    updateUserInfo();
                    alert('头像更新成功！');
                } else {
                    alert('头像上传失败: ' + (result.error || '未知错误'));
                }
            } catch (error) {
                alert('头像上传失败: ' + error.message);
            }

            input.value = '';
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
                            msg_type: msg.data.msg_type || 1,
                            file_id: msg.data.file_id,
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
                
                // 头像（暂时用默认头像，可以后续从session中获取）
                let avatarHTML;
                if (session.avatar) {
                    avatarHTML = '<img src="' + session.avatar + '" class="session-avatar">';
                } else {
                    avatarHTML = '<div class="session-avatar default">U</div>';
                }
                
                div.innerHTML = 
                    avatarHTML +
                    '<div class="session-info">' +
                        '<div class="session-name">用户 ' + session.target_id + '</div>' +
                        '<div class="session-last-msg">' + (session.last_message || '暂无消息') + '</div>' +
                    '</div>';
                div.onclick = () => selectUser(session.target_id, 'User ' + session.target_id);
                list.appendChild(div);
            });
        }

        // 选择用户
        async function selectUser(userId, nickname) {
            // 先获取对方用户信息
            const userInfoResult = await apiCall('/api/user/info?user_id=' + userId, null, token);
            if (userInfoResult.code === 200) {
                currentTargetUser = {
                    id: userId,
                    nickname: userInfoResult.data.nickname || nickname,
                    avatar: userInfoResult.data.avatar,
                    signature: userInfoResult.data.signature
                };
            } else {
                currentTargetUser = { id: userId, nickname: nickname };
            }
            
            document.getElementById('chatTitle').textContent = currentTargetUser.nickname;
            document.getElementById('chatMessages').innerHTML = '';
            
            // 加载历史消息
            const result = await apiCall('/api/messages?target_id=' + userId + '&limit=50', null, token);
            if (result.code === 200) {
                const messages = result.data || [];
                messages.reverse().forEach(msg => {
                    displayMessage({
                        content: msg.content,
                        msg_type: msg.msg_type || 1,
                        file_id: msg.file_id,
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
                msg_type: 1,
                isSent: true,
                time: Date.now()
            });
        }

        // 处理文件选择
        async function handleFileSelect(input, fileType) {
            if (!currentTargetUser) {
                alert('请先选择聊天对象');
                input.value = '';
                return;
            }

            const file = input.files[0];
            if (!file) return;

            // 文件大小限制检查（10MB）
            if (file.size > 10 * 1024 * 1024) {
                alert('文件大小不能超过 10MB');
                input.value = '';
                return;
            }

            // 显示上传中的占位消息
            const uploadingDiv = displayUploadingMessage(file.name, fileType);

            try {
                // 上传文件
                const formData = new FormData();
                formData.append('file', file);

                const response = await fetch('/api/upload/' + fileType, {
                    method: 'POST',
                    headers: {
                        'Authorization': 'Bearer ' + token
                    },
                    body: formData
                });

                const result = await response.json();

                // 移除上传中的占位消息
                uploadingDiv.remove();

                if (result.code === 200) {
                    const fileInfo = result.data;
                    
                    // 确定消息类型
                    let msgType = 1;
                    if (fileType === 'image') msgType = 2;
                    else if (fileType === 'video') msgType = 3;
                    else if (fileType === 'voice') msgType = 4;
                    else if (fileType === 'file') msgType = 5;

                    // 发送文件消息
                    const msgId = generateUUID();
                    const msg = {
                        type: 'chat_msg',
                        msg_id: msgId,
                        data: {
                            msg_id: msgId,
                            to_user_id: currentTargetUser.id,
                            content: file.name,
                            msg_type: msgType,
                            file_id: fileInfo.file_id,
                            client_time: Date.now()
                        },
                        timestamp: Date.now()
                    };

                    ws.send(JSON.stringify(msg));

                    // 显示发送的文件消息
                    displayMessage({
                        content: file.name,
                        msg_type: msgType,
                        file_info: fileInfo,
                        isSent: true,
                        time: Date.now()
                    });
                } else {
                    alert('上传失败: ' + (result.error || '未知错误'));
                }
            } catch (error) {
                uploadingDiv.remove();
                alert('上传失败: ' + error.message);
            }

            input.value = '';
        }

        // 显示上传中的消息
        function displayUploadingMessage(filename, fileType) {
            const messagesDiv = document.getElementById('chatMessages');
            const msgDiv = document.createElement('div');
            msgDiv.className = 'message sent uploading';

            const contentDiv = document.createElement('div');
            contentDiv.className = 'message-content';
            
            let icon = '📄';
            if (fileType === 'image') icon = '📷';
            else if (fileType === 'video') icon = '🎬';
            else if (fileType === 'voice') icon = '🎤';
            
            contentDiv.innerHTML = '<div>' + icon + ' ' + filename + '</div>';

            msgDiv.appendChild(contentDiv);
            messagesDiv.appendChild(msgDiv);
            messagesDiv.scrollTop = messagesDiv.scrollHeight;

            return msgDiv;
        }

        // 显示消息
        function displayMessage(msg) {
            const messagesDiv = document.getElementById('chatMessages');
            const msgDiv = document.createElement('div');
            msgDiv.className = 'message ' + (msg.isSent ? 'sent' : 'received');

            // 添加头像
            const avatarDiv = document.createElement('div');
            if (msg.isSent) {
                // 发送者头像（当前用户）
                if (currentUser.avatar) {
                    avatarDiv.innerHTML = '<img src="' + currentUser.avatar + '" class="message-avatar">';
                } else {
                    avatarDiv.innerHTML = '<div class="message-avatar default">' + currentUser.nickname.charAt(0).toUpperCase() + '</div>';
                }
            } else {
                // 接收者头像（对方用户）
                if (currentTargetUser && currentTargetUser.avatar) {
                    avatarDiv.innerHTML = '<img src="' + currentTargetUser.avatar + '" class="message-avatar">';
                } else {
                    const initial = currentTargetUser ? currentTargetUser.nickname.charAt(0).toUpperCase() : '?';
                    avatarDiv.innerHTML = '<div class="message-avatar default">' + initial + '</div>';
                }
            }

            const contentDiv = document.createElement('div');
            contentDiv.className = 'message-content';

            // 根据消息类型渲染不同内容
            const msgType = msg.msg_type || 1;
            
            if (msgType === 1) {
                // 文本消息
                contentDiv.textContent = msg.content;
            } else if (msgType === 2) {
                // 图片消息
                const img = document.createElement('img');
                img.className = 'message-image';
                img.src = msg.file_info ? msg.file_info.url : '/api/files/' + msg.file_id;
                img.alt = msg.content;
                img.onclick = () => window.open(img.src, '_blank');
                contentDiv.appendChild(img);
            } else if (msgType === 3) {
                // 视频消息
                const video = document.createElement('video');
                video.className = 'message-video';
                video.controls = true;
                video.src = msg.file_info ? msg.file_info.url : '/api/files/' + msg.file_id;
                contentDiv.appendChild(video);
            } else if (msgType === 4) {
                // 语音消息
                const voiceDiv = document.createElement('div');
                voiceDiv.className = 'message-voice';
                voiceDiv.innerHTML = '<span>🎤</span>';
                
                const audio = document.createElement('audio');
                audio.controls = true;
                audio.src = msg.file_info ? msg.file_info.url : '/api/files/' + msg.file_id;
                voiceDiv.appendChild(audio);
                contentDiv.appendChild(voiceDiv);
            } else if (msgType === 5) {
                // 文件消息
                const fileDiv = document.createElement('div');
                fileDiv.className = 'message-file';
                fileDiv.innerHTML = '<span class="file-icon">📎</span><span>' + msg.content + '</span>';
                fileDiv.onclick = () => {
                    const url = msg.file_info ? msg.file_info.url : '/api/files/' + msg.file_id;
                    window.open(url, '_blank');
                };
                fileDiv.style.cursor = 'pointer';
                contentDiv.appendChild(fileDiv);
            }

            const infoDiv = document.createElement('div');
            infoDiv.className = 'message-info';
            infoDiv.textContent = new Date(msg.time).toLocaleTimeString();

            contentDiv.appendChild(infoDiv);
            msgDiv.appendChild(avatarDiv);
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
