package repository

import (
	"time"

	"gorm.io/gorm"
)

// DBServer 服务器节点数据库模型
type DBServer struct {
	ServerID      string    `gorm:"primaryKey;type:varchar(64)"`
	GRPCAddr      string    `gorm:"column:grpc_addr;type:varchar(128);not null"`
	LastHeartbeat int64     `gorm:"index:idx_heartbeat;not null"`
	CreatedAt     time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`
}

func (DBServer) TableName() string {
	return "im_servers"
}

// DBUserRoute 用户路由数据库模型
type DBUserRoute struct {
	UserID        int64     `gorm:"primaryKey;autoIncrement:false"`
	ServerID      string    `gorm:"type:varchar(64);index:idx_server;not null"`
	LastHeartbeat int64     `gorm:"index:idx_heartbeat;not null"`
	CreatedAt     time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"`
}

func (DBUserRoute) TableName() string {
	return "im_user_routes"
}

// Server 服务器节点模型
type Server struct {
	ServerID      string
	GRPCAddr      string
	LastHeartbeat int64
}

// RouteRepository 路由仓库
type RouteRepository struct {
	db *gorm.DB
}

// NewRouteRepository 创建路由仓库
func NewRouteRepository(db *gorm.DB) *RouteRepository {
	return &RouteRepository{db: db}
}

// InitTables 初始化数据库表
func (r *RouteRepository) InitTables() error {
	return r.db.AutoMigrate(&DBServer{}, &DBUserRoute{})
}

// RegisterServer 注册服务器节点
func (r *RouteRepository) RegisterServer(serverID, grpcAddr string) error {
	now := time.Now().Unix()
	
	// 先尝试更新
	result := r.db.Model(&DBServer{}).
		Where("server_id = ?", serverID).
		Updates(map[string]interface{}{
			"grpc_addr":      grpcAddr,
			"last_heartbeat": now,
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	// 如果没有更新到记录，说明不存在，需要插入
	if result.RowsAffected == 0 {
		server := &DBServer{
			ServerID:      serverID,
			GRPCAddr:      grpcAddr,
			LastHeartbeat: now,
		}
		return r.db.Create(server).Error
	}
	
	return nil
}

// UnregisterServer 注销服务器节点
func (r *RouteRepository) UnregisterServer(serverID string) error {
	return r.db.Delete(&DBServer{}, "server_id = ?", serverID).Error
}

// UpdateServerHeartbeat 更新服务器心跳
func (r *RouteRepository) UpdateServerHeartbeat(serverID string) error {
	now := time.Now().Unix()
	return r.db.Model(&DBServer{}).
		Where("server_id = ?", serverID).
		Update("last_heartbeat", now).Error
}

// GetActiveServers 获取活跃的服务器列表
func (r *RouteRepository) GetActiveServers() ([]*Server, error) {
	var dbServers []DBServer
	timeout := time.Now().Unix() - 60 // 60秒内有心跳的认为在线

	if err := r.db.Where("last_heartbeat > ?", timeout).Find(&dbServers).Error; err != nil {
		return nil, err
	}

	servers := make([]*Server, len(dbServers))
	for i, s := range dbServers {
		servers[i] = &Server{
			ServerID:      s.ServerID,
			GRPCAddr:      s.GRPCAddr,
			LastHeartbeat: s.LastHeartbeat,
		}
	}

	return servers, nil
}

// RegisterUserRoute 注册用户路由
func (r *RouteRepository) RegisterUserRoute(userID int64, serverID string) error {
	now := time.Now().Unix()
	
	// 先尝试更新
	result := r.db.Model(&DBUserRoute{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"server_id":      serverID,
			"last_heartbeat": now,
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	// 如果没有更新到记录，说明不存在，需要插入
	if result.RowsAffected == 0 {
		route := &DBUserRoute{
			UserID:        userID,
			ServerID:      serverID,
			LastHeartbeat: now,
		}
		return r.db.Create(route).Error
	}
	
	return nil
}

// UnregisterUserRoute 注销用户路由
func (r *RouteRepository) UnregisterUserRoute(userID int64) error {
	return r.db.Delete(&DBUserRoute{}, "user_id = ?", userID).Error
}

// UserRoute 用户路由结果
type UserRoute struct {
	ServerID  string
	GRPCAddr  string
}

// GetUserRoute 获取用户路由
func (r *RouteRepository) GetUserRoute(userID int64) (*UserRoute, error) {
	var route DBUserRoute
	if err := r.db.Where("user_id = ?", userID).First(&route).Error; err != nil {
		return nil, err
	}

	// 查询服务器信息
	var server DBServer
	if err := r.db.Where("server_id = ?", route.ServerID).First(&server).Error; err != nil {
		return nil, err
	}

	return &UserRoute{
		ServerID: route.ServerID,
		GRPCAddr: server.GRPCAddr,
	}, nil
}

// BatchUpdateHeartbeat 批量更新用户心跳
func (r *RouteRepository) BatchUpdateHeartbeat(userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}

	now := time.Now().Unix()
	return r.db.Model(&DBUserRoute{}).
		Where("user_id IN ?", userIDs).
		Update("last_heartbeat", now).Error
}
