package core

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Hub WebSocket 连接管理中心
type Hub struct {
	clients   map[int64]*Client
	mutex     sync.RWMutex
	broadcast chan *BroadcastMessage
}

// Client 客户端连接
type Client struct {
	UserID int64
	Conn   *websocket.Conn
	Send   chan []byte
}

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	UserIDs []int64
	Data    []byte
}

// NewHub 创建 Hub
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[int64]*Client),
		broadcast: make(chan *BroadcastMessage, 256),
	}
}

// Run 启动 Hub
func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mutex.RLock()
		for _, userID := range msg.UserIDs {
			if client, ok := h.clients[userID]; ok {
				select {
				case client.Send <- msg.Data:
				default:
					// 发送失败，关闭连接
					close(client.Send)
					delete(h.clients, userID)
				}
			}
		}
		h.mutex.RUnlock()
	}
}

// Register 注册客户端
func (h *Hub) Register(userID int64, conn *websocket.Conn) *Client {
	client := &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	h.mutex.Lock()
	// 如果用户已存在连接，关闭旧连接
	if oldClient, exists := h.clients[userID]; exists {
		close(oldClient.Send)
		oldClient.Conn.Close()
	}
	h.clients[userID] = client
	h.mutex.Unlock()

	// 启动写协程
	go client.writePump()

	return client
}

// Unregister 注销客户端
func (h *Hub) Unregister(userID int64) {
	h.mutex.Lock()
	if client, ok := h.clients[userID]; ok {
		close(client.Send)
		client.Conn.Close()
		delete(h.clients, userID)
	}
	h.mutex.Unlock()
}

// SendToUser 发送消息给指定用户
func (h *Hub) SendToUser(userID int64, data []byte) bool {
	h.mutex.RLock()
	client, exists := h.clients[userID]
	h.mutex.RUnlock()

	if !exists {
		return false
	}

	select {
	case client.Send <- data:
		return true
	default:
		return false
	}
}

// SendToUsers 发送消息给多个用户
func (h *Hub) SendToUsers(userIDs []int64, data []byte) {
	h.broadcast <- &BroadcastMessage{
		UserIDs: userIDs,
		Data:    data,
	}
}

// HasClient 检查用户是否在线
func (h *Hub) HasClient(userID int64) bool {
	h.mutex.RLock()
	_, exists := h.clients[userID]
	h.mutex.RUnlock()
	return exists
}

// GetOnlineUsers 获取所有在线用户
func (h *Hub) GetOnlineUsers() []int64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	userIDs := make([]int64, 0, len(h.clients))
	for userID := range h.clients {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// writePump 写协程
func (c *Client) writePump() {
	defer func() {
		c.Conn.Close()
	}()

	for data := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}
