-- IM 模块数据库表结构

-- 服务器节点表
CREATE TABLE IF NOT EXISTS im_servers (
    server_id VARCHAR(64) PRIMARY KEY COMMENT '服务器节点 ID',
    grpc_addr VARCHAR(128) NOT NULL COMMENT 'gRPC 地址',
    last_heartbeat BIGINT NOT NULL COMMENT '最后心跳时间戳（秒）',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_heartbeat (last_heartbeat)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='IM 服务器节点表';

-- 用户路由表
CREATE TABLE IF NOT EXISTS im_user_routes (
    user_id BIGINT PRIMARY KEY COMMENT '用户 ID',
    server_id VARCHAR(64) NOT NULL COMMENT '所在服务器节点 ID',
    last_heartbeat BIGINT NOT NULL COMMENT '最后心跳时间戳（秒）',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '连接建立时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_server (server_id),
    INDEX idx_heartbeat (last_heartbeat)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户路由表';

-- 消息表
CREATE TABLE IF NOT EXISTS im_messages (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '自增 ID',
    msg_id VARCHAR(64) UNIQUE NOT NULL COMMENT '消息唯一 ID',
    from_user_id BIGINT NOT NULL COMMENT '发送者用户 ID',
    to_user_id BIGINT NOT NULL COMMENT '接收者用户 ID',
    group_id BIGINT DEFAULT 0 COMMENT '群组 ID（0 表示单聊）',
    content TEXT NOT NULL COMMENT '消息内容',
    msg_type TINYINT DEFAULT 1 COMMENT '消息类型（1:文本 2:图片 3:语音 4:视频 5:文件）',
    status TINYINT DEFAULT 1 COMMENT '消息状态（1:发送中 2:已发送 3:已送达 4:已读 5:失败）',
    client_time BIGINT COMMENT '客户端时间戳（毫秒）',
    server_time BIGINT NOT NULL COMMENT '服务端时间戳（毫秒）',
    delivered_time BIGINT DEFAULT 0 COMMENT '送达时间戳（毫秒）',
    read_time BIGINT DEFAULT 0 COMMENT '已读时间戳（毫秒）',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX idx_msg_id (msg_id),
    INDEX idx_from (from_user_id, server_time DESC),
    INDEX idx_to (to_user_id, status, server_time DESC),
    INDEX idx_group (group_id, server_time DESC),
    INDEX idx_server_time (server_time DESC, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消息表';

-- 会话表
CREATE TABLE IF NOT EXISTS im_sessions (
    user_id BIGINT NOT NULL COMMENT '用户 ID',
    target_id BIGINT NOT NULL COMMENT '对方用户 ID 或群组 ID',
    session_type TINYINT DEFAULT 1 COMMENT '会话类型（1:单聊 2:群聊）',
    last_msg_content TEXT COMMENT '最后一条消息内容',
    last_msg_time BIGINT COMMENT '最后消息时间戳（毫秒）',
    unread_count INT DEFAULT 0 COMMENT '未读消息数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (user_id, target_id, session_type),
    INDEX idx_user_time (user_id, last_msg_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话表';

-- 群组表
CREATE TABLE IF NOT EXISTS im_groups (
    group_id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '群组 ID',
    group_name VARCHAR(100) NOT NULL COMMENT '群组名称',
    owner_id BIGINT NOT NULL COMMENT '群主用户 ID',
    avatar_url VARCHAR(255) COMMENT '群头像 URL',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组表';

-- 群成员表
CREATE TABLE IF NOT EXISTS im_group_members (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '自增 ID',
    group_id BIGINT NOT NULL COMMENT '群组 ID',
    user_id BIGINT NOT NULL COMMENT '用户 ID',
    role TINYINT DEFAULT 0 COMMENT '角色（0:普通成员 1:管理员 2:群主）',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '加入时间',
    UNIQUE KEY uk_group_user (group_id, user_id),
    INDEX idx_user (user_id),
    INDEX idx_group (group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群成员表';
