package models

import "gorm.io/gorm"

// 添加对gorm包的显式引用（用于消除未使用导入警告）
var _ = gorm.ErrRecordNotFound

// File 文件主表结构
type File struct {
	ID         string `gorm:"primaryKey;size:47" json:"id"`
	Name       string `gorm:"size:240" json:"name"`
	Version    int    `gorm:"not null" json:"version"`
	Size       int    `gorm:"not null" json:"size"`
	CreateTime int64  `gorm:"not null" json:"create_time"`
	ModifyTime int64  `gorm:"not null" json:"modify_time"`
	CreatorID  string `gorm:"size:48;not null" json:"creator_id"`
	ModifierID string `gorm:"size:48;not null" json:"modifier_id"`
}

// FileVersion 文件版本历史
type FileVersion struct {
	ID         string `gorm:"primaryKey;size:47" json:"id"`
	Version    int    `gorm:"not null" json:"version"`
	Name       string `gorm:"size:240" json:"name"`
	Size       int    `gorm:"not null" json:"size"`
	CreateTime int64  `gorm:"not null" json:"create_time"`
	ModifierID string `gorm:"size:48;not null" json:"modifier_id"`
}

// User 用户信息
type User struct {
	ID        string `gorm:"primaryKey;size:48" json:"id"`
	Name      string `gorm:"size:100" json:"name"`
	AvatarURL string `gorm:"size:200" json:"avatar_url,omitempty"`
}

// Watermark 水印配置
type Watermark struct {
	FileID     string `gorm:"primaryKey;size:47" json:"file_id"`
	Type       int    `gorm:"not null" json:"type"`
	Value      string `gorm:"size:200" json:"value"`
	Horizontal int    `gorm:"not null" json:"horizontal"`
	Vertical   int    `gorm:"not null" json:"vertical"`
}

// Attachment 附件存储
type Attachment struct {
	Key       string `gorm:"primaryKey;size:100" json:"key"`
	Data      []byte `gorm:"type:longblob" json:"-"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
}
