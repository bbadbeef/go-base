# Storage 模块

文件存储模块，支持将文件存储到数据库中。适用于小文件（<10MB）的多节点部署场景。

## 特性

- ✅ 文件存储到数据库（支持多节点部署，无需同步）
- ✅ 支持图片、视频、语音、普通文件
- ✅ 文件大小限制（最大 10MB）
- ✅ MIME 类型验证
- ✅ 软删除支持

## 安装

```bash
go get github.com/bbadbeef/go-base/storage
```

## 使用示例

```go
package main

import (
    "github.com/bbadbeef/go-base/storage"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func main() {
    // 连接数据库
    db, _ := gorm.Open(mysql.Open("root:password@tcp(localhost:3306)/test"))
    
    // 创建存储实例
    st, err := storage.NewStorage(&storage.Config{
        DB:      db,
        BaseURL: "http://localhost:8080",
    })
    
    // 上传文件
    fileInfo, err := st.Upload(&storage.UploadRequest{
        File:     file,
        Header:   fileHeader,
        UserID:   123,
        FileType: storage.FileTypeImage,
    })
    
    // 下载文件
    data, fileInfo, err := st.Download(fileID)
    
    // 删除文件
    err = st.Delete(fileID)
}
```

## 支持的文件类型

### 图片 (image)
- JPEG (.jpg, .jpeg)
- PNG (.png)
- GIF (.gif)
- WebP (.webp)
- BMP (.bmp)

### 视频 (video)
- MP4 (.mp4)
- QuickTime (.mov)
- AVI (.avi)
- MPEG (.mpeg)

### 语音 (voice)
- MP3 (.mp3)
- WAV (.wav)
- OGG (.ogg)
- AAC (.aac)
- M4A (.m4a)

## 文件大小限制

所有文件类型最大支持 10MB。

## 数据库表结构

表名：`storage_files`

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 自增主键 |
| file_id | VARCHAR(64) | 文件唯一ID |
| user_id | BIGINT | 上传用户ID |
| file_name | VARCHAR(255) | 原始文件名 |
| file_type | VARCHAR(50) | 文件类型 |
| mime_type | VARCHAR(100) | MIME类型 |
| file_size | BIGINT | 文件大小（字节） |
| file_data | MEDIUMBLOB | 文件二进制数据 |
| status | TINYINT | 状态（1:正常 2:已删除） |
| created_at | TIMESTAMP | 创建时间 |
