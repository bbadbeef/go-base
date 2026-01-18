package core

import (
	"sync"
	"time"

	"github.com/bbadbeef/go-base/im/internal/repository"
)

// RouteManager 路由管理器
type RouteManager struct {
	serverID  string
	routeRepo *repository.RouteRepository
	cacheTTL  int

	// 本地缓存
	userRoutes   map[int64]*RouteCache
	gatewayAddrs map[string]string
	mutex        sync.RWMutex
}

// RouteCache 路由缓存
type RouteCache struct {
	GatewayID string
	CacheTime int64
}

// NewRouteManager 创建路由管理器
func NewRouteManager(serverID string, routeRepo *repository.RouteRepository, cacheTTL int) *RouteManager {
	return &RouteManager{
		serverID:     serverID,
		routeRepo:    routeRepo,
		cacheTTL:     cacheTTL,
		userRoutes:   make(map[int64]*RouteCache),
		gatewayAddrs: make(map[string]string),
	}
}

// Register 注册用户路由
func (rm *RouteManager) Register(userID int64, gatewayID string) error {
	// 写入数据库
	if err := rm.routeRepo.RegisterUserRoute(userID, gatewayID); err != nil {
		return err
	}

	// 更新本地缓存
	rm.mutex.Lock()
	rm.userRoutes[userID] = &RouteCache{
		GatewayID: gatewayID,
		CacheTime: time.Now().Unix(),
	}
	rm.mutex.Unlock()

	return nil
}

// Unregister 注销用户路由
func (rm *RouteManager) Unregister(userID int64) error {
	// 从数据库删除
	if err := rm.routeRepo.UnregisterUserRoute(userID); err != nil {
		return err
	}

	// 清理本地缓存
	rm.mutex.Lock()
	delete(rm.userRoutes, userID)
	rm.mutex.Unlock()

	return nil
}

// GetUserRoute 获取用户路由
// 返回: gatewayID, gatewayAddr, online
func (rm *RouteManager) GetUserRoute(userID int64) (string, string, bool) {
	// 1. 查本地缓存
	rm.mutex.RLock()
	if route, exists := rm.userRoutes[userID]; exists {
		if time.Now().Unix()-route.CacheTime < int64(rm.cacheTTL) {
			addr := rm.gatewayAddrs[route.GatewayID]
			rm.mutex.RUnlock()
			return route.GatewayID, addr, true
		}
	}
	rm.mutex.RUnlock()

	// 2. 缓存未命中或过期，查询数据库
	userRoute, err := rm.routeRepo.GetUserRoute(userID)
	if err != nil {
		return "", "", false
	}

	// 3. 更新本地缓存
	rm.mutex.Lock()
	rm.userRoutes[userID] = &RouteCache{
		GatewayID: userRoute.ServerID,
		CacheTime: time.Now().Unix(),
	}
	rm.gatewayAddrs[userRoute.ServerID] = userRoute.GRPCAddr
	rm.mutex.Unlock()

	return userRoute.ServerID, userRoute.GRPCAddr, true
}

// BatchUpdateHeartbeat 批量更新用户心跳
func (rm *RouteManager) BatchUpdateHeartbeat(userIDs []int64) error {
	return rm.routeRepo.BatchUpdateHeartbeat(userIDs)
}
