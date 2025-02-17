package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gorm.io/gorm/clause" // 新增引用
)

// ---------------------------
// 配置项（生产环境应使用环境变量）
// ---------------------------
const (
	dbUser     = "webuser"
	dbPassword = "Aa@123456"
	dbHost     = "localhost"
	dbPort     = "3306"
	dbName     = "weboffice"

	storageBaseURL = "http://storage.example.com"
	uploadEndpoint = "http://upload.example.com"
)

// ---------------------------
// 数据库模型（严格遵循JSON规范）
// ---------------------------
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

type FileVersion struct {
	ID         string `gorm:"primaryKey;size:47" json:"id"`
	Version    int    `gorm:"not null" json:"version"`
	Name       string `gorm:"size:240" json:"name"`
	Size       int    `gorm:"not null" json:"size"`
	CreateTime int64  `gorm:"not null" json:"create_time"`
	ModifierID string `gorm:"size:48;not null" json:"modifier_id"`
}

type User struct {
	ID        string `gorm:"primaryKey;size:48" json:"id"`
	Name      string `gorm:"size:100" json:"name"`
	AvatarURL string `gorm:"size:200" json:"avatar_url,omitempty"`
}

type Watermark struct {
	FileID     string `gorm:"primaryKey;size:47" json:"file_id"`
	Type       int    `gorm:"not null" json:"type"`
	Value      string `gorm:"size:200" json:"value"`
	Horizontal int    `gorm:"not null" json:"horizontal"`
	Vertical   int    `gorm:"not null" json:"vertical"`
}

type Attachment struct {
	Key       string `gorm:"primaryKey;size:100" json:"key"`
	Data      []byte `gorm:"type:longblob" json:"-"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
}

// ---------------------------
// 全局上下文
// ---------------------------
var db *gorm.DB

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ---------------------------
// 数据库初始化（带连接池配置）
// ---------------------------
func initDB() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true, // 开启预编译
	})

	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := db.AutoMigrate(&File{}, &FileVersion{}, &User{}, &Watermark{}, &Attachment{}); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	return nil
}

// ---------------------------
// 核心回调接口实现
// ---------------------------

// GET /v3/3rd/files/:file_id
func handleGetFile(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	if fileID == "" {
		respondError(c, 400, "Invalid file ID")
		return
	}

	var file File
	if err := db.Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, 404, "File not found")
		} else {
			respondError(c, 500, "Database error")
		}
		return
	}

	// 强制一致性校验
	if file.ID != fileID {
		log.Printf("CRITICAL: File ID mismatch (request:%s db:%s)", fileID, file.ID)
		respondError(c, 500, "Data inconsistency")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: file,
	})
}

// GET /v3/3rd/files/:file_id/download
func handleGetDownloadURL(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	if fileID == "" {
		respondError(c, 400, "Invalid file ID")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: map[string]string{
			"url": fmt.Sprintf("%s/files/%s", storageBaseURL, fileID),
		},
	})
}

// GET /v3/3rd/files/:file_id/permission
func handleGetPermissions(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	if fileID == "" {
		respondError(c, 400, "Invalid file ID")
		return
	}

	var file File
	if err := db.Where("id = ?", fileID).First(&file).Error; err != nil {
		respondError(c, 404, "File not found")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: map[string]interface{}{
			"read":        1,
			"update":      1,
			"download":    1,
			"user_id":     file.CreatorID,
			"history":     1,
			"copy":        1,
			"print":       1,
			"modifier_id": file.ModifierID,
		},
	})
}

// GET /v3/3rd/files/:file_id/upload/prepare
func handlePrepareUpload(c *gin.Context) {
	c.JSON(200, Response{
		Code: 0,
		Data: map[string]interface{}{
			"digest_types": []string{"sha1", "md5"},
		},
	})
}

// POST /v3/3rd/files/:file_id/upload/address
func handleGetUploadAddress(c *gin.Context) {
	var req struct {
		Name     string            `json:"name"`
		Size     int               `json:"size"`
		Digest   map[string]string `json:"digest"`
		IsManual bool              `json:"is_manual"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, 400, "Invalid request body")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: map[string]interface{}{
			"url":    fmt.Sprintf("%s/upload", uploadEndpoint),
			"method": "PUT",
			"headers": map[string]string{
				"Content-Type": "application/octet-stream",
			},
		},
	})
}

// POST /v3/3rd/files/:file_id/upload/complete
func handleUploadComplete(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	if fileID == "" {
		respondError(c, 400, "Invalid file ID")
		return
	}

	var req struct {
		Request  map[string]interface{} `json:"request"`
		Response map[string]interface{} `json:"response"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, 400, "Invalid request body")
		return
	}

	// 使用事务处理版本更新
	err := db.Transaction(func(tx *gorm.DB) error {
		// 获取当前文件
		var file File
		if err := tx.Where("id = ?", fileID).First(&file).Error; err != nil {
			return err
		}

		// 创建新版本记录
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

		// 更新主文件
		updateFields := map[string]interface{}{
			"version":     newVersion.Version,
			"modify_time": time.Now().Unix(),
			"size":        req.Request["size"],
		}

		if err := tx.Model(&File{}).Where("id = ?", fileID).Updates(updateFields).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		log.Printf("Upload complete failed: %v", err)
		respondError(c, 500, "Failed to process upload")
		return
	}

	c.JSON(200, Response{Code: 0})
}

// GET /v3/3rd/users
func handleGetUsers(c *gin.Context) {
	userIDs := c.QueryArray("user_ids")
	if len(userIDs) == 0 {
		respondError(c, 400, "Missing user_ids parameter")
		return
	}

	var users []User
	if err := db.Where("id IN (?)", userIDs).Find(&users).Error; err != nil {
		respondError(c, 500, "Database error")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: users,
	})
}

// PUT /v3/3rd/files/:file_id/name
func handleRenameFile(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	if fileID == "" {
		respondError(c, 400, "Invalid file ID")
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, 400, "Invalid request body")
		return
	}

	if err := db.Model(&File{}).Where("id = ?", fileID).Update("name", req.Name).Error; err != nil {
		respondError(c, 500, "Failed to update filename")
		return
	}

	c.JSON(200, Response{Code: 0})
}

// GET /v3/3rd/files/:file_id/versions
func handleListVersions(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var versions []FileVersion
	if err := db.Where("id = ?", fileID).
		Order("version DESC").
		Offset(offset).
		Limit(limit).
		Find(&versions).Error; err != nil {
		respondError(c, 500, "Database error")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: versions,
	})
}

// GET /v3/3rd/files/:file_id/versions/:version
func handleGetVersion(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		respondError(c, 400, "Invalid version number")
		return
	}

	var versionData FileVersion
	if err := db.Where("id = ? AND version = ?", fileID, version).First(&versionData).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, 404, "Version not found")
		} else {
			respondError(c, 500, "Database error")
		}
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: versionData,
	})
}

// GET /v3/3rd/files/:file_id/watermark
func handleGetWatermark(c *gin.Context) {
	fileID := sanitizeID(c.Param("file_id"))
	if fileID == "" {
		respondError(c, 400, "Invalid file ID")
		return
	}

	var watermark Watermark
	if err := db.Where("file_id = ?", fileID).First(&watermark).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(200, Response{
				Code: 0,
				Data: map[string]interface{}{"type": 0},
			})
		} else {
			respondError(c, 500, "Database error")
		}
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: watermark,
	})
}

// PUT /v3/3rd/object/:key
func handleUploadObject(c *gin.Context) {
	key := c.Param("key")
	data, err := c.GetRawData()
	if err != nil {
		respondError(c, 400, "Failed to read object data")
		return
	}

	// 计算MD5校验和
	hash := md5.Sum(data)
	digest := hex.EncodeToString(hash[:])

	attachment := Attachment{
		Key:       key,
		Data:      data,
		CreatedAt: time.Now().Unix(),
	}

	if err := db.Create(&attachment).Error; err != nil {
		respondError(c, 500, "Failed to store object")
		return
	}

	c.JSON(200, Response{
		Code: 0,
		Data: map[string]string{
			"digest": digest,
		},
	})
}

// GET /v3/3rd/object/:key/url
func handleGetObjectURL(c *gin.Context) {
	key := c.Param("key")
	c.JSON(200, Response{
		Code: 0,
		Data: map[string]string{
			"url": fmt.Sprintf("%s/objects/%s", storageBaseURL, key),
		},
	})
}

// POST /v3/3rd/object/copy
func handleCopyObject(c *gin.Context) {
	var req struct {
		KeyDict map[string]string `json:"key_dict"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, 400, "Invalid request body")
		return
	}

	// 使用事务处理复制操作
	err := db.Transaction(func(tx *gorm.DB) error {
		for srcKey, dstKey := range req.KeyDict {
			var src Attachment
			if err := tx.Where("key = ?", srcKey).First(&src).Error; err != nil {
				return fmt.Errorf("source object %s not found", srcKey)
			}

			dst := Attachment{
				Key:       dstKey,
				Data:      src.Data,
				CreatedAt: time.Now().Unix(),
			}

			if err := tx.Create(&dst).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		respondError(c, 500, "Failed to copy objects: "+err.Error())
		return
	}

	c.JSON(200, Response{Code: 0})
}

// ---------------------------
// 工具函数
// ---------------------------
func sanitizeID(id string) string {
	return strings.TrimSpace(id)
}

func respondError(c *gin.Context, code int, message string) {
	c.JSON(200, Response{
		Code:    code,
		Message: message,
	})
}

// ---------------------------
// 主函数
// ---------------------------
func main() {
	// 初始化数据库
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 初始化测试数据
	if err := initTestData(); err != nil {
		log.Fatalf("Test data initialization failed: %v", err)
	}

	// 配置Gin路由
	r := gin.Default()
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.ErrorMessage,
		)
	}))

	// 注册所有回调接口
	registerRoutes(r)

	// 启动服务
	log.Println("Starting server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}

func registerRoutes(r *gin.Engine) {
	// 文件相关
	r.GET("/v3/3rd/files/:file_id", handleGetFile)
	r.GET("/v3/3rd/files/:file_id/download", handleGetDownloadURL)
	r.GET("/v3/3rd/files/:file_id/permission", handleGetPermissions)

	// 三阶段保存
	r.GET("/v3/3rd/files/:file_id/upload/prepare", handlePrepareUpload)
	r.POST("/v3/3rd/files/:file_id/upload/address", handleGetUploadAddress)
	r.POST("/v3/3rd/files/:file_id/upload/complete", handleUploadComplete)

	// 用户相关
	r.GET("/v3/3rd/users", handleGetUsers)

	// 文件操作
	r.PUT("/v3/3rd/files/:file_id/name", handleRenameFile)
	r.GET("/v3/3rd/files/:file_id/versions", handleListVersions)
	r.GET("/v3/3rd/files/:file_id/versions/:version", handleGetVersion)
	r.GET("/v3/3rd/files/:file_id/versions/:version/download", handleGetDownloadURL)

	// 水印配置
	r.GET("/v3/3rd/files/:file_id/watermark", handleGetWatermark)

	// 附件处理
	r.PUT("/v3/3rd/object/:key", handleUploadObject)
	r.GET("/v3/3rd/object/:key/url", handleGetObjectURL)
	r.POST("/v3/3rd/object/copy", handleCopyObject)
}

// ---------------------------
// 测试数据初始化（增强版：幂等性设计）
// ---------------------------
func initTestData() error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 清理旧测试数据（关键修改点）
		if err := tx.Exec("DELETE FROM users WHERE id = ?", "user1").Error; err != nil {
			return fmt.Errorf("清理用户数据失败: %w", err)
		}

		if err := tx.Exec("DELETE FROM files WHERE id = ?", "file123").Error; err != nil {
			return fmt.Errorf("清理文件数据失败: %w", err)
		}

		if err := tx.Exec("DELETE FROM watermarks WHERE file_id = ?", "file123").Error; err != nil {
			return fmt.Errorf("清理水印数据失败: %w", err)
		}

		if err := tx.Exec("DELETE FROM attachments WHERE key = ?", "sample_key").Error; err != nil {
			return fmt.Errorf("清理附件数据失败: %w", err)
		}

		// 初始化用户（使用 Clauses 处理冲突）
		user := User{
			ID:        "user1",
			Name:      "Admin",
			AvatarURL: "https://example.com/avatar.jpg",
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}}, // 根据主键判断冲突
			DoUpdates: clause.Assignments(map[string]interface{}{ // 冲突时更新字段
				"name":       user.Name,
				"avatar_url": user.AvatarURL,
			}),
		}).Create(&user).Error; err != nil {
			return fmt.Errorf("初始化用户失败: %w", err)
		}

		// 初始化主文件（使用 Upsert）
		now := time.Now().Unix()
		file := File{
			ID:         "file123",
			Name:       "测试文档.docx",
			Version:    1,
			Size:       1024,
			CreateTime: now,
			ModifyTime: now,
			CreatorID:  "user1",
			ModifierID: "user1",
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true, // 冲突时更新所有字段
		}).Create(&file).Error; err != nil {
			return fmt.Errorf("初始化文件失败: %w", err)
		}

		// 初始化水印（使用 CreateOrUpdate 模式）
		watermark := Watermark{
			FileID:     "file123",
			Type:       1,
			Value:      "Confidential",
			Horizontal: 50,
			Vertical:   100,
		}
		if err := tx.Where(Watermark{FileID: "file123"}).
			Assign(watermark). // 存在则更新，不存在则插入
			FirstOrCreate(&watermark).Error; err != nil {
			return fmt.Errorf("初始化水印失败: %w", err)
		}

		// 初始化附件（带数据校验）
		attachment := Attachment{
			Key:       "sample_key",
			Data:      []byte("sample content"),
			CreatedAt: now,
		}
		if len(attachment.Data) == 0 {
			return errors.New("附件内容不能为空")
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoNothing: true, // 冲突时不执行操作
		}).Create(&attachment).Error; err != nil {
			return fmt.Errorf("初始化附件失败: %w", err)
		}

		return nil
	})
}
