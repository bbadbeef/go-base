package core

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"

	imgrpc "github.com/bbadbeef/go-base/im/internal/grpc"
	"github.com/bbadbeef/go-base/im/internal/log"
	"github.com/bbadbeef/go-base/im/internal/model"
	"github.com/bbadbeef/go-base/im/internal/protocol"
	"github.com/bbadbeef/go-base/im/internal/repository"
	"github.com/bbadbeef/go-base/im/internal/util"
)

// IMServer IM 服务器实现
type IMServer struct {
	config *Config

	// 连接管理
	hub *Hub

	// 路由管理
	routeManager *RouteManager

	// 节点间通信
	grpcServer  *grpc.Server
	peerClients map[string]imgrpc.IMServerClient
	peerMutex   sync.RWMutex

	// 数据访问
	messageRepo *repository.MessageRepository
	routeRepo   *repository.RouteRepository
	sessionRepo *repository.SessionRepository

	// 回调函数
	onMessageHandlers     []func(*model.Message)
	onUserOnlineHandlers  []func(int64)
	onUserOfflineHandlers []func(int64)

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// NewIMServer 创建 IM 服务器实例
func NewIMServer(config *Config) (*IMServer, error) {
	s := &IMServer{
		config:      config,
		hub:         NewHub(),
		peerClients: make(map[string]imgrpc.IMServerClient),
	}

	// 初始化数据访问层
	s.messageRepo = repository.NewMessageRepository(config.DB)
	s.routeRepo = repository.NewRouteRepository(config.DB)
	s.sessionRepo = repository.NewSessionRepository(config.DB)

	// 自动创建表
	if err := s.messageRepo.InitTables(); err != nil {
		return nil, err
	}
	if err := s.routeRepo.InitTables(); err != nil {
		return nil, err
	}
	if err := s.sessionRepo.InitTables(); err != nil {
		return nil, err
	}

	// 初始化路由管理器
	s.routeManager = NewRouteManager(config.ServerID, s.routeRepo, config.CacheTTL)

	return s, nil
}

// Start 启动 IM 服务
func (s *IMServer) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// 1. 注册当前节点
	if err := s.registerNode(); err != nil {
		return err
	}

	// 2. 启动连接管理器
	go s.hub.Run()

	// 3. 启动心跳
	go s.heartbeatWorker()

	// 4. 启动 gRPC Server（节点间通信）
	if s.config.GRPCAddr != "" {
		go s.startGRPCServer()
	}

	// 5. 发现其他节点并建立连接
	go s.discoverPeers()

	log.Infof("Server started, id=%s", s.config.ServerID)

	<-s.ctx.Done()
	return nil
}

// Stop 停止 IM 服务
func (s *IMServer) Stop() error {
	log.Infof("Server stopping...")

	// 1. 注销节点
	s.unregisterNode()

	// 2. 停止上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 3. 关闭 gRPC
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	log.Infof("Server stopped")
	return nil
}

// WebSocketHandler 获取 WebSocket Handler
func (s *IMServer) WebSocketHandler() http.HandlerFunc {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 获取 Token
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		// 2. 调用主应用的认证函数
		userID, err := s.config.AuthFunc(token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// 3. 升级为 WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("Failed to upgrade websocket: %v", err)
			return
		}

		// 4. 处理连接
		s.onUserConnect(userID, conn)
	}
}

// SendMessage 发送消息（主动推送，如系统消息）
func (s *IMServer) SendMessage(ctx context.Context, req *model.SendMessageRequest) error {
	msg := &model.Message{
		MsgID:      util.GenerateMsgID(),
		FromUserID: req.FromUserID,
		ToUserID:   req.ToUserID,
		GroupID:    req.GroupID,
		Content:    req.Content,
		MsgType:    req.MsgType,
		Status:     model.MsgStatusSent,
		ServerTime: time.Now().UnixMilli(),
	}

	// 1. 持久化
	if err := s.messageRepo.Save(msg); err != nil {
		return err
	}

	// 2. 更新会话
	s.updateSession(msg)

	// 3. 路由转发
	return s.routeAndDeliver(msg)
}

// IsUserOnline 检查用户是否在线
func (s *IMServer) IsUserOnline(userID int64) bool {
	return s.hub.HasClient(userID)
}

// GetSessions 获取会话列表
func (s *IMServer) GetSessions(ctx context.Context, userID int64) ([]*model.Session, error) {
	return s.sessionRepo.GetUserSessions(userID)
}

// GetMessages 获取历史消息
func (s *IMServer) GetMessages(ctx context.Context, req *model.GetMessagesRequest) ([]*model.Message, error) {
	if req.Limit == 0 {
		req.Limit = 20
	}
	return s.messageRepo.GetMessages(req)
}

// MarkAsRead 标记消息为已读
func (s *IMServer) MarkAsRead(ctx context.Context, userID int64, msgIDs []string) error {
	readTime := time.Now().UnixMilli()

	for _, msgID := range msgIDs {
		// 更新消息状态
		if err := s.messageRepo.UpdateStatus(msgID, model.MsgStatusRead, readTime); err != nil {
			log.Warnf("Failed to mark message as read: %v", err)
			continue
		}

		// 查询消息的发送方
		msg, err := s.messageRepo.GetByMsgID(msgID)
		if err != nil {
			continue
		}

		// 通知发送方
		s.notifyStatusUpdate(msg.FromUserID, msgID, model.MsgStatusRead, readTime)
	}

	return nil
}

// OnMessage 设置消息回调
func (s *IMServer) OnMessage(handler func(*model.Message)) {
	s.onMessageHandlers = append(s.onMessageHandlers, handler)
}

// OnUserOnline 设置用户上线回调
func (s *IMServer) OnUserOnline(handler func(int64)) {
	s.onUserOnlineHandlers = append(s.onUserOnlineHandlers, handler)
}

// OnUserOffline 设置用户下线回调
func (s *IMServer) OnUserOffline(handler func(int64)) {
	s.onUserOfflineHandlers = append(s.onUserOfflineHandlers, handler)
}

// ========== 内部实现方法 ==========

// 用户连接处理
func (s *IMServer) onUserConnect(userID int64, conn *websocket.Conn) {
	log.Infof("User connected: %d", userID)

	// 1. 注册到 Hub
	client := s.hub.Register(userID, conn)

	// 2. 更新路由表
	s.routeManager.Register(userID, s.config.ServerID)

	// 3. 触发上线回调
	for _, handler := range s.onUserOnlineHandlers {
		go handler(userID)
	}

	// 4. 推送离线消息（如果有）
	go s.pushOfflineMessages(userID)

	// 5. 启动消息处理
	go s.handleClientMessages(client)
}

// 用户断开处理
func (s *IMServer) onUserDisconnect(userID int64) {
	log.Infof("User disconnected: %d", userID)

	// 1. 从 Hub 移除
	s.hub.Unregister(userID)

	// 2. 更新路由表
	s.routeManager.Unregister(userID)

	// 3. 触发下线回调
	for _, handler := range s.onUserOfflineHandlers {
		go handler(userID)
	}
}

// 处理客户端消息
func (s *IMServer) handleClientMessages(client *Client) {
	defer s.onUserDisconnect(client.UserID)

	for {
		var wsMsg protocol.WSMessage
		if err := client.Conn.ReadJSON(&wsMsg); err != nil {
			log.Debugf("Read error from user %d: %v", client.UserID, err)
			break
		}

		log.Debugf("Received message type: %s from user %d", wsMsg.Type, client.UserID)

		switch wsMsg.Type {
		case protocol.WSMsgTypePing:
			s.handlePing(client)
		case protocol.WSMsgTypeChatMsg:
			s.handleChatMessage(client.UserID, &wsMsg)
		case protocol.WSMsgTypeGroupMsg:
			s.handleGroupMessage(client.UserID, &wsMsg)
		case protocol.WSMsgTypeReadReceipt:
			s.handleReadReceipt(client.UserID, &wsMsg)
		case protocol.WSMsgTypeDeliveredReceipt:
			s.handleDeliveredReceipt(client.UserID, &wsMsg)
		default:
			log.Warnf("Unknown message type: %s from user %d", wsMsg.Type, client.UserID)
		}
	}
}

// 处理心跳
func (s *IMServer) handlePing(client *Client) {
	pong := &protocol.WSMessage{
		Type:      protocol.WSMsgTypePong,
		Timestamp: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(pong)
	client.Send <- data
}

// 处理聊天消息
func (s *IMServer) handleChatMessage(fromUserID int64, wsMsg *protocol.WSMessage) {
	log.Debugf("handleChatMessage from user %d", fromUserID)
	
	var chatMsg protocol.WSChatMessage
	data, _ := json.Marshal(wsMsg.Data)
	log.Debugf("Message data: %s", string(data))
	
	if err := json.Unmarshal(data, &chatMsg); err != nil {
		log.Errorf("Invalid chat message from user %d: %v", fromUserID, err)
		return
	}

	// 如果客户端没有提供 msg_id，服务器生成一个
	if chatMsg.MsgID == "" {
		chatMsg.MsgID = util.GenerateMsgID()
		log.Debugf("Generated msg_id: %s", chatMsg.MsgID)
	}

	log.Debugf("Chat message: msgID=%s, toUserID=%d", chatMsg.MsgID, chatMsg.ToUserID)

	serverTime := time.Now().UnixMilli()

	// 创建消息
	msg := &model.Message{
		MsgID:      chatMsg.MsgID,
		FromUserID: fromUserID,
		ToUserID:   chatMsg.ToUserID,
		Content:    chatMsg.Content,
		MsgType:    chatMsg.MsgType,
		FileID:     chatMsg.FileID,
		Status:     model.MsgStatusSent,
		ClientTime: chatMsg.ClientTime,
		ServerTime: serverTime,
	}

	// 1. 持久化
	if err := s.messageRepo.Save(msg); err != nil {
		log.Errorf("Failed to save message %s: %v", msg.MsgID, err)
		s.sendAck(fromUserID, chatMsg.MsgID, model.MsgStatusFailed, err.Error())
		return
	}

	log.Infof("Message saved: %s (%d -> %d)", msg.MsgID, msg.FromUserID, msg.ToUserID)

	// 2. 发送 ACK
	s.sendAck(fromUserID, chatMsg.MsgID, model.MsgStatusSent, "")

	// 3. 更新会话
	s.updateSession(msg)

	// 4. 触发回调
	for _, handler := range s.onMessageHandlers {
		go handler(msg)
	}

	// 5. 路由转发
	s.routeAndDeliver(msg)
}

// 处理群聊消息
func (s *IMServer) handleGroupMessage(fromUserID int64, wsMsg *protocol.WSMessage) {
	// TODO: 实现群聊消息处理
	log.Warnf("Group message not implemented yet")
}

// 处理已读回执
func (s *IMServer) handleReadReceipt(userID int64, wsMsg *protocol.WSMessage) {
	var receipt protocol.WSReceipt
	data, _ := json.Marshal(wsMsg.Data)
	if err := json.Unmarshal(data, &receipt); err != nil {
		return
	}

	s.MarkAsRead(context.Background(), userID, []string{receipt.MsgID})
}

// 处理送达回执
func (s *IMServer) handleDeliveredReceipt(userID int64, wsMsg *protocol.WSMessage) {
	var receipt protocol.WSReceipt
	data, _ := json.Marshal(wsMsg.Data)
	if err := json.Unmarshal(data, &receipt); err != nil {
		return
	}

	deliveredTime := time.Now().UnixMilli()

	// 更新消息状态
	if err := s.messageRepo.UpdateStatus(receipt.MsgID, model.MsgStatusDelivered, deliveredTime); err != nil {
		return
	}

	// 查询消息的发送方
	msg, err := s.messageRepo.GetByMsgID(receipt.MsgID)
	if err != nil {
		return
	}

	// 通知发送方
	s.notifyStatusUpdate(msg.FromUserID, receipt.MsgID, model.MsgStatusDelivered, deliveredTime)
}

// 发送 ACK
func (s *IMServer) sendAck(userID int64, msgID string, status int, errMsg string) {
	ack := &protocol.WSMessage{
		Type:      protocol.WSMsgTypeAck,
		MsgID:     msgID,
		Timestamp: time.Now().UnixMilli(),
		Data: &protocol.WSAckMessage{
			MsgID:      msgID,
			Status:     status,
			ServerTime: time.Now().UnixMilli(),
			Error:      errMsg,
		},
	}

	data, _ := json.Marshal(ack)
	s.hub.SendToUser(userID, data)
}

// 通知状态更新
func (s *IMServer) notifyStatusUpdate(userID int64, msgID string, status int, updateTime int64) {
	update := &protocol.WSMessage{
		Type:      protocol.WSMsgTypeStatusUpdate,
		MsgID:     msgID,
		Timestamp: updateTime,
		Data: &protocol.WSStatusUpdate{
			MsgID:      msgID,
			Status:     status,
			UpdateTime: updateTime,
		},
	}

	data, _ := json.Marshal(update)
	s.hub.SendToUser(userID, data)
}

// 路由并投递消息（核心转发逻辑）
func (s *IMServer) routeAndDeliver(msg *model.Message) error {
	// 查询接收方路由
	gatewayID, gatewayAddr, online := s.routeManager.GetUserRoute(msg.ToUserID)

	if !online {
		log.Debugf("User %d offline, message saved", msg.ToUserID)
		return nil
	}

	if gatewayID == s.config.ServerID {
		// 本地推送
		log.Debugf("Delivering message locally to user %d", msg.ToUserID)
		s.pushToLocalUser(msg)
	} else {
		// 远程转发到其他节点
		log.Debugf("Forwarding message to remote gateway %s", gatewayID)
		s.forwardToRemoteGateway(gatewayAddr, msg)
	}

	return nil
}

// 本地推送
func (s *IMServer) pushToLocalUser(msg *model.Message) {
	pushMsg := &protocol.WSMessage{
		Type:      protocol.WSMsgTypeChatMsg,
		MsgID:     msg.MsgID,
		Timestamp: msg.ServerTime,
		Data: &protocol.WSPushMessage{
			MsgID:      msg.MsgID,
			FromUserID: msg.FromUserID,
			Content:    msg.Content,
			MsgType:    msg.MsgType,
			FileID:     msg.FileID,
			Status:     msg.Status,
			ClientTime: msg.ClientTime,
			ServerTime: msg.ServerTime,
		},
	}

	data, _ := json.Marshal(pushMsg)
	delivered := s.hub.SendToUser(msg.ToUserID, data)

	if delivered {
		// 自动更新为已送达
		deliveredTime := time.Now().UnixMilli()
		s.messageRepo.UpdateStatus(msg.MsgID, model.MsgStatusDelivered, deliveredTime)
		s.notifyStatusUpdate(msg.FromUserID, msg.MsgID, model.MsgStatusDelivered, deliveredTime)
		log.Debugf("Message %s delivered to user %d", msg.MsgID, msg.ToUserID)
	} else {
		log.Warnf("Failed to deliver message %s to user %d", msg.MsgID, msg.ToUserID)
	}
}

// 远程转发（节点间通信）
func (s *IMServer) forwardToRemoteGateway(addr string, msg *model.Message) {
	s.peerMutex.RLock()
	client, exists := s.peerClients[addr]
	s.peerMutex.RUnlock()

	if !exists {
		log.Debugf("No peer client for %s, attempting to connect", addr)
		// 尝试建立连接
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Errorf("Failed to connect to peer %s: %v", addr, err)
			return
		}
		client = imgrpc.NewIMServerClient(conn)
		s.peerMutex.Lock()
		s.peerClients[addr] = client
		s.peerMutex.Unlock()
	}

	// 转发消息
	req := imgrpc.MessageToForwardRequest(msg)
	resp, err := client.ForwardMessage(context.Background(), req)
	if err != nil {
		log.Errorf("Failed to forward message: %v", err)
		return
	}

	if resp.Delivered {
		log.Debugf("Message %s forwarded successfully", msg.MsgID)
	} else {
		log.Errorf("Message %s forward failed: %s", msg.MsgID, resp.Error)
	}
}

// 推送离线消息
func (s *IMServer) pushOfflineMessages(userID int64) {
	// 1. 查询该用户的未送达消息
	messages, err := s.messageRepo.GetUndeliveredMessages(userID, 100)
	if err != nil {
		log.Errorf("Failed to get offline messages for user %d: %v", userID, err)
		return
	}

	if len(messages) == 0 {
		log.Debugf("No offline messages for user %d", userID)
		return
	}

	log.Infof("Pushing %d offline messages to user %d", len(messages), userID)

	// 2. 批量推送
	for _, msg := range messages {
		pushMsg := &protocol.WSMessage{
			Type:      protocol.WSMsgTypeChatMsg,
			MsgID:     msg.MsgID,
			Timestamp: msg.ServerTime,
			Data: &protocol.WSPushMessage{
				MsgID:      msg.MsgID,
				FromUserID: msg.FromUserID,
				Content:    msg.Content,
				MsgType:    msg.MsgType,
				FileID:     msg.FileID,
				Status:     msg.Status,
				ClientTime: msg.ClientTime,
				ServerTime: msg.ServerTime,
			},
		}

		data, _ := json.Marshal(pushMsg)
		delivered := s.hub.SendToUser(userID, data)

		if delivered {
			// 更新为已送达
			deliveredTime := time.Now().UnixMilli()
			s.messageRepo.UpdateStatus(msg.MsgID, model.MsgStatusDelivered, deliveredTime)
			
			// 通知发送方
			s.notifyStatusUpdate(msg.FromUserID, msg.MsgID, model.MsgStatusDelivered, deliveredTime)
			
			log.Debugf("Offline message %s delivered to user %d", msg.MsgID, userID)
		} else {
			log.Warnf("Failed to deliver offline message %s to user %d", msg.MsgID, userID)
			break // 如果一条消息发送失败，停止发送后续消息
		}

		// 避免一次性发送过多，稍微延迟
		time.Sleep(10 * time.Millisecond)
	}

	log.Debugf("Finished pushing offline messages to user %d", userID)
}

// 更新会话
func (s *IMServer) updateSession(msg *model.Message) {
	// 更新发送方会话
	s.sessionRepo.UpdateSession(&model.Session{
		UserID:         msg.FromUserID,
		TargetID:       msg.ToUserID,
		SessionType:    model.SessionTypeSingle,
		LastMsgContent: msg.Content,
		LastMsgTime:    msg.ServerTime,
	})

	// 更新接收方会话（增加未读数）
	s.sessionRepo.UpdateSession(&model.Session{
		UserID:         msg.ToUserID,
		TargetID:       msg.FromUserID,
		SessionType:    model.SessionTypeSingle,
		LastMsgContent: msg.Content,
		LastMsgTime:    msg.ServerTime,
		UnreadCount:    1, // 会累加
	})
}

// 注册节点
func (s *IMServer) registerNode() error {
	return s.routeRepo.RegisterServer(s.config.ServerID, s.config.GRPCAddr)
}

// 注销节点
func (s *IMServer) unregisterNode() {
	s.routeRepo.UnregisterServer(s.config.ServerID)
}

// 心跳工作器
func (s *IMServer) heartbeatWorker() {
	ticker := time.NewTicker(time.Duration(s.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 更新服务器心跳
			s.routeRepo.UpdateServerHeartbeat(s.config.ServerID)

			// 批量更新在线用户心跳
			userIDs := s.hub.GetOnlineUsers()
			if len(userIDs) > 0 {
				s.routeManager.BatchUpdateHeartbeat(userIDs)
			}
		}
	}
}

// 启动 gRPC Server
func (s *IMServer) startGRPCServer() {
	lis, err := net.Listen("tcp", s.config.GRPCAddr)
	if err != nil {
		log.Fatalf("Failed to listen gRPC: %v", err)
		return
	}

	s.grpcServer = grpc.NewServer()
	imgrpc.RegisterIMServerServer(s.grpcServer, s)

	log.Infof("gRPC server listening on %s", s.config.GRPCAddr)

	if err := s.grpcServer.Serve(lis); err != nil {
		log.Errorf("gRPC server error: %v", err)
	}
}

// 发现其他节点
func (s *IMServer) discoverPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			servers, err := s.routeRepo.GetActiveServers()
			if err != nil {
				continue
			}

			for _, server := range servers {
				if server.ServerID == s.config.ServerID {
					continue
				}

				s.peerMutex.Lock()
				if _, exists := s.peerClients[server.ServerID]; !exists {
					// 建立新连接
					conn, err := grpc.Dial(server.GRPCAddr, grpc.WithInsecure())
					if err != nil {
						log.Errorf("Failed to connect to peer %s: %v", server.ServerID, err)
						s.peerMutex.Unlock()
						continue
					}
					s.peerClients[server.ServerID] = imgrpc.NewIMServerClient(conn)
					log.Infof("Connected to peer: %s", server.ServerID)
				}
				s.peerMutex.Unlock()
			}
		}
	}
}

// ForwardMessage gRPC 服务端实现（接收其他节点转发的消息）
func (s *IMServer) ForwardMessage(ctx context.Context, req *imgrpc.ForwardMessageRequest) (*imgrpc.ForwardMessageResponse, error) {
	log.Debugf("Received forwarded message %s from remote gateway", req.MsgID)

	// 推送给本地用户
	msg := &model.Message{
		MsgID:      req.MsgID,
		FromUserID: req.FromUserID,
		ToUserID:   req.ToUserID,
		Content:    req.Content,
		MsgType:    int(req.MsgType),
		Status:     model.MsgStatusSent,
		ClientTime: req.ClientTime,
		ServerTime: req.ServerTime,
	}

	s.pushToLocalUser(msg)

	return &imgrpc.ForwardMessageResponse{
		Delivered: true,
	}, nil
}
