package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ---------------------------
// 数据库配置
// ---------------------------
const (
	dbUser     = "root"
	dbPassword = "Aa@123456"
	dbHost     = "120.78.90.3"
	dbPort     = "3306"
	dbName     = "weboffice"
)

// ---------------------------
// 数据库模型定义
// ---------------------------
type File struct {
	ID         string `gorm:"primaryKey;size:47"`
	Name       string `gorm:"size:240"`
	Version    int    `gorm:"not null"`
	Size       int    `gorm:"not null"`
	CreateTime int64  `gorm:"not null"`
	ModifyTime int64  `gorm:"not null"`
	CreatorID  string `gorm:"size:48;not null"`
	ModifierID string `gorm:"size:48;not null"`
}

type FileVersion struct {
	ID         string `gorm:"primaryKey;size:47"`
	Version    int    `gorm:"not null"` // 修正：只保留ID作为主键
	Name       string `gorm:"size:240"`
	Size       int    `gorm:"not null"`
	CreateTime int64  `gorm:"not null"`
	ModifierID string `gorm:"size:48;not null"`
}

type User struct {
	ID        string `gorm:"primaryKey;size:48"`
	Name      string `gorm:"size:100"`
	AvatarURL string `gorm:"size:200"`
}

type Attachment struct {
	Key       string `gorm:"primaryKey;size:100"`
	Data      []byte `gorm:"type:longblob"`
	CreatedAt int64  `gorm:"not null"`
}

type Watermark struct {
	FileID     string `gorm:"primaryKey;size:47"`
	Type       int    `gorm:"not null"`
	Value      string `gorm:"size:200"`
	Horizontal int    `gorm:"not null"`
	Vertical   int    `gorm:"not null"`
}

// ---------------------------
// 全局变量
// ---------------------------
var db *gorm.DB

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ---------------------------
// 数据库初始化
// ---------------------------
func initDB() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("Failed to connect database: %w", err)
	}

	// 自动迁移表结构
	err = db.AutoMigrate(&File{}, &FileVersion{}, &User{}, &Attachment{}, &Watermark{})
	if err != nil {
		return fmt.Errorf("AutoMigrate failed: %w", err)
	}

	return nil
}

// ---------------------------
// 处理函数实现
// ---------------------------

// 获取文件信息
func handleGetFileInfo(c *gin.Context) {
	fileID := c.Param("file_id")
	var file File
	result := db.First(&file, "id =?", fileID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, Response{Code: 404, Message: "File not found"})
		return
	}
	c.JSON(http.StatusOK, Response{Code: 0, Data: file})
}

// 获取文件下载地址
func handleGetDownloadURL(c *gin.Context) {
	fileID := c.Param("file_id")
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Data: map[string]string{
			"url": fmt.Sprintf("http://storage.example.com/files/%s", fileID),
		},
	})
}

// 获取文件权限
func handleGetPermissions(c *gin.Context) {
	fileID := c.Param("file_id")

	// 查询文件信息
	var file File
	result := db.First(&file, "id = ?", fileID)
	if result.Error != nil {
		c.JSON(http.StatusOK, Response{
			Code: 0,
			Data: map[string]interface{}{
				"read":     0,
				"update":   0,
				"download": 0,
				"user_id":  "",
			},
		})
		return
	}

	// 示例逻辑：文件创建者有完全权限
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Data: map[string]interface{}{
			"read":     1,
			"update":   1,
			"download": 1,
			"user_id":  file.CreatorID,
		},
	})
}

// 三阶段保存 - 准备上传
func handlePrepareUpload(c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Data: map[string]interface{}{
			"digest_types": []string{"sha1"},
		},
	})
}

// 三阶段保存 - 获取上传地址
func handleGetUploadAddress(c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Data: map[string]string{
			"url":    "http://upload.example.com/files",
			"method": "PUT",
		},
	})
}

// 三阶段保存 - 完成上传
func handleUploadComplete(c *gin.Context) {
	fileID := c.Param("file_id")

	// 使用事务处理
	err := db.Transaction(func(tx *gorm.DB) error {
		// 更新主文件
		var file File
		if err := tx.First(&file, "id =?", fileID).Error; err != nil {
			return err
		}

		// 创建新版本
		newVersion := FileVersion{
			ID:         fileID,
			Version:    file.Version + 1,
			Name:       file.Name,
			Size:       file.Size,
			CreateTime: time.Now().Unix(),
			ModifierID: file.ModifierID,
		}
		if err := tx.Create(&newVersion).Error; err != nil {
			return err
		}

		// 更新主文件版本
		if err := tx.Model(&file).Update("version", newVersion.Version).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: fmt.Sprintf("Internal server error: %v", err)})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0})
}

// 获取用户信息
func handleGetUsers(c *gin.Context) {
	userIDs := c.QueryArray("user_ids")
	if len(userIDs) == 0 {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "user_ids parameter is required"})
		return
	}

	var users []User
	result := db.Where("id IN?", userIDs).Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Data: users})
}

// 文件重命名
func handleRenameFile(c *gin.Context) {
	fileID := c.Param("file_id")
	var req struct{ Name string }
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "Invalid request"})
		return
	}

	result := db.Model(&File{}).Where("id =?", fileID).Update("name", req.Name)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0})
}

// 获取文件版本列表
func handleGetVersions(c *gin.Context) {
	fileID := c.Param("file_id")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	var versions []FileVersion
	result := db.Where("id =?", fileID).
		Order("version DESC").
		Offset(offset).
		Limit(limit).
		Find(&versions)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Data: versions})
}

// 获取指定版本
func handleGetVersion(c *gin.Context) {
	fileID := c.Param("file_id")
	versionStr := c.Param("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "Invalid version"})
		return
	}

	var versionData FileVersion
	result := db.Where("id =? AND version =?", fileID, version).First(&versionData)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, Response{Code: 404})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0, Data: versionData})
}

// 获取版本下载地址
func handleGetVersionDownload(c *gin.Context) {
	fileID := c.Param("file_id")
	versionStr := c.Param("version")
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Data: map[string]string{
			"url": fmt.Sprintf("http://storage.example.com/files/%s/versions/%s", fileID, versionStr),
		},
	})
}

// 获取水印配置
func handleGetWatermark(c *gin.Context) {
	fileID := c.Param("file_id")
	var watermark Watermark

	// 使用 fileID 查询水印配置
	result := db.Where("file_id = ?", fileID).First(&watermark)
	if result.Error != nil {
		c.JSON(http.StatusOK, Response{
			Code: 0,
			Data: gin.H{"type": 0}, // 默认无水印
		})
		return
	}
	c.JSON(http.StatusOK, Response{Code: 0, Data: watermark})
}

// 上传附件
func handleUploadAttachment(c *gin.Context) {
	key := c.Param("key")
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "Invalid data"})
		return
	}

	attachment := Attachment{
		Key:       key,
		Data:      data,
		CreatedAt: time.Now().Unix(),
	}
	result := db.Create(&attachment)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, Response{Code: 500})
		return
	}

	c.JSON(http.StatusOK, Response{Code: 0})
}

// 获取附件下载地址
func handleGetAttachmentURL(c *gin.Context) {
	key := c.Param("key")
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Data: map[string]string{
			"url": fmt.Sprintf("http://storage.example.com/object/%s", key),
		},
	})
}

// 复制附件
func handleCopyAttachment(c *gin.Context) {
	var req struct {
		KeyDict map[string]string `json:"key_dict"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Code: 400, Message: "Invalid request"})
		return
	}

	for srcKey, dstKey := range req.KeyDict {
		var attachment Attachment
		result := db.First(&attachment, "key =?", srcKey)
		if result.Error != nil {
			continue
		}

		newAttachment := Attachment{
			Key:       dstKey,
			Data:      attachment.Data,
			CreatedAt: time.Now().Unix(),
		}
		db.Create(&newAttachment)
	}

	c.JSON(http.StatusOK, Response{Code: 0})
}

// ---------------------------
// 主函数
// ---------------------------
func main() {
	// 初始化数据库
	if err := initDB(); err != nil {
		log.Fatal(err)
	}

	// 初始化测试数据
	initTestData()

	// 初始化Gin
	r := gin.Default()

	// 中间件
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// 路由设置
	r.GET("/v3/3rd/files/:file_id", handleGetFileInfo)
	r.GET("/v3/3rd/files/:file_id/download", handleGetDownloadURL)
	r.GET("/v3/3rd/files/:file_id/permission", handleGetPermissions)
	r.GET("/v3/3rd/files/:file_id/upload/prepare", handlePrepareUpload)
	r.POST("/v3/3rd/files/:file_id/upload/address", handleGetUploadAddress)
	r.POST("/v3/3rd/files/:file_id/upload/complete", handleUploadComplete)
	r.GET("/v3/3rd/users", handleGetUsers)
	r.PUT("/v3/3rd/files/:file_id/name", handleRenameFile)
	r.GET("/v3/3rd/files/:file_id/versions", handleGetVersions)
	r.GET("/v3/3rd/files/:file_id/versions/:version", handleGetVersion)
	r.GET("/v3/3rd/files/:file_id/versions/:version/download", handleGetVersionDownload)
	r.GET("/v3/3rd/files/:file_id/watermark", handleGetWatermark)
	r.PUT("/v3/3rd/object/:key", handleUploadAttachment)
	r.GET("/v3/3rd/object/:key/url", handleGetAttachmentURL)
	r.POST("/v3/3rd/object/copy", handleCopyAttachment)

	// 启动服务
	log.Println("Server started on :8080")
	log.Fatal(r.Run(":8080"))
}

// ---------------------------
// 测试数据初始化
// ---------------------------
func initTestData() {
	// 初始化用户
	db.Create(&User{
		ID:        "user1",
		Name:      "Admin",
		AvatarURL: "https://example.com/avatar.jpg",
	})

	// 初始化文件
	db.Create(&File{
		ID:         "file123",
		Name:       "测试文档.docx",
		Version:    1,
		Size:       1024,
		CreateTime: time.Now().Unix(),
		ModifyTime: time.Now().Unix(),
		CreatorID:  "user1",
		ModifierID: "user1",
	})

	// 初始化水印配置
	db.Create(&Watermark{
		FileID:     "file123",
		Type:       1,
		Value:      "Confidential",
		Horizontal: 50,
		Vertical:   100,
	})
}
