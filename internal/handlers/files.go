package handlers

import (
	"errors"
	"fmt"
	"io" // 新增导入
	"log"
	"net/http"
	"path/filepath" // 新增导入
	"strconv"
	"time"

	"net/url" // 新增导入：用于文件名编码
	"os"      // 新增导入：用于文件存在性检查

	"gorm.io/gorm/clause" // 新增导入

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"weboffice/internal/config" // 新增导入
	"weboffice/internal/database"
	"weboffice/internal/models"
	"weboffice/internal/storage" // 新增导入
	"weboffice/internal/utils"
)

// 添加全局存储实例
var fileStorage *storage.FileStorage

// InitFileStorage 正确类型声明
func InitFileStorage(s *storage.FileStorage) {
	fileStorage = s
}

// GetFile 处理获取文件元数据
func GetFile(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	if fileID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var file models.File
	if err := database.DB.Where("id = ?", fileID).First(&file).Error; err != nil {
		handleDatabaseError(c, err)
		return
	}

	if file.ID != fileID {
		log.Printf("CRITICAL: File ID mismatch (request:%s db:%s)", fileID, file.ID)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Data inconsistency")
		return
	}

	utils.SuccessResponse(c, file)
}

// GetDownloadURL 处理获取下载地址
// GetDownloadURL 修复后的函数
func GetDownloadURL(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	if fileID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	// 从配置系统获取存储地址
	cfg := config.LoadConfig()
	utils.SuccessResponse(c, gin.H{
		"url": fmt.Sprintf("%s/files/%s", cfg.StorageURL, fileID),
	})
}

// GetPermissions 处理获取文件权限
func GetPermissions(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	if fileID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var file models.File
	if err := database.DB.Where("id = ?", fileID).First(&file).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "File not found")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"read":        1,
		"update":      1,
		"download":    1,
		"user_id":     file.CreatorID,
		"history":     1,
		"copy":        1,
		"print":       1,
		"modifier_id": file.ModifierID,
	})
}

// PrepareUpload 处理上传准备
func PrepareUpload(c *gin.Context) {
	utils.SuccessResponse(c, gin.H{
		"digest_types": []string{"sha1", "md5"},
	})
}

// GetUploadAddress 处理获取上传地址
// GetUploadAddress 修复后的函数
func GetUploadAddress(c *gin.Context) {
	var req struct {
		Name     string            `json:"name"`
		Size     int               `json:"size"`
		Digest   map[string]string `json:"digest"`
		IsManual bool              `json:"is_manual"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 从配置系统获取上传地址
	cfg := config.LoadConfig()
	utils.SuccessResponse(c, gin.H{
		"url":    fmt.Sprintf("%s/upload", cfg.UploadURL),
		"method": "PUT",
		"headers": map[string]string{
			"Content-Type": "application/octet-stream",
		},
	})
}

// UploadComplete 处理上传完成
func UploadComplete(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	if fileID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "无效的文件ID")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "缺少文件内容")
		return
	}

	if err := utils.ValidateFileType(fileHeader); err != nil {
		utils.ErrorResponse(c, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("打开上传文件失败: %v", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "文件处理失败")
		return
	}
	defer file.Close()

	var (
		currentVersion int
		currentUserID  = "user1" // 实际应从认证信息获取
		fileName       = filepath.Base(fileHeader.Filename)
	)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 行级锁查询主文件记录
		var fileModel models.File
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", fileID).
			First(&fileModel)

		// 2. 处理文件不存在的情况
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// 3. 原子化检查残留版本记录
				var versionExists bool
				if err := tx.Model(&models.FileVersion{}).
					Select("count(*) > 0").
					Where("id = ? AND version = 1", fileID).
					Find(&versionExists).Error; err != nil {
					return fmt.Errorf("版本存在性检查失败: %w", err)
				}
				if versionExists {
					return fmt.Errorf("检测到残留版本记录，请更换文件ID")
				}

				// 4. 创建主文件记录
				now := time.Now().Unix()
				newFile := models.File{
					ID:         fileID,
					Name:       fileName,
					Version:    1,
					Size:       int(fileHeader.Size),
					CreateTime: now,
					ModifyTime: now,
					CreatorID:  currentUserID,
					ModifierID: currentUserID,
				}
				if err := tx.Create(&newFile).Error; err != nil {
					return fmt.Errorf("创建主文件记录失败: %w", err)
				}

				// 5. 安全创建初始版本记录
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "id"}, {Name: "version"}},
					DoNothing: true,
				}).Create(&models.FileVersion{
					ID:         fileID,
					Version:    1,
					Name:       fileName,
					Size:       int(fileHeader.Size),
					CreateTime: now,
					ModifierID: currentUserID,
				}).Error; err != nil {
					tx.Rollback() // 强制回滚主文件记录
					return fmt.Errorf("创建版本记录失败: %w", err)
				}

				currentVersion = 1
			} else {
				return fmt.Errorf("查询文件失败: %w", result.Error)
			}
		} else {
			// 6. 处理文件已存在的情况
			// 原子递增版本号
			if err := tx.Model(&models.File{}).
				Where("id = ?", fileID).
				Update("version", gorm.Expr("version + 1")).Error; err != nil {
				return fmt.Errorf("版本递增失败: %w", err)
			}

			// 7. 获取最新版本号
			var latestFile models.File
			if err := tx.Select("version").
				Where("id = ?", fileID).
				First(&latestFile).Error; err != nil {
				return fmt.Errorf("获取最新版本失败: %w", err)
			}
			currentVersion = latestFile.Version

			// 8. 创建新版本记录
			newVersion := models.FileVersion{
				ID:         fileID,
				Version:    currentVersion,
				Name:       fileName,
				Size:       int(fileHeader.Size),
				CreateTime: time.Now().Unix(),
				ModifierID: currentUserID,
			}
			if err := tx.Create(&newVersion).Error; err != nil {
				return fmt.Errorf("创建版本记录失败: %w", err)
			}

			// 9. 清理旧版本（保留最近5个）
			if currentVersion > 5 {
				if err := tx.Where("id = ? AND version < ?",
					fileID, currentVersion-5).
					Delete(&models.FileVersion{}).Error; err != nil {
					log.Printf("版本清理失败（非致命错误）: %v", err)
				}
			}
		}

		// 10. 更新主文件元数据
		updateFields := map[string]interface{}{
			"name":        fileName,
			"modify_time": time.Now().Unix(),
			"size":        int(fileHeader.Size),
			"modifier_id": currentUserID,
		}
		if err := tx.Model(&models.File{}).
			Where("id = ?", fileID).
			Updates(updateFields).Error; err != nil {
			return fmt.Errorf("更新文件失败: %w", err)
		}

		// 11. 存储文件内容
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("文件指针重置失败: %w", err)
		}
		if err := fileStorage.SaveFile(fileID, currentVersion, fileName, file); err != nil {
			return fmt.Errorf("文件存储失败: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("上传处理失败: %v", err)
		utils.ErrorResponse(c, http.StatusInternalServerError,
			fmt.Sprintf("上传处理失败: %v", err))
		return
	}

	utils.SuccessResponse(c, gin.H{
		"message":   "上传完成",
		"version":   currentVersion,
		"file_id":   fileID,
		"file_name": fileName,
	})
}

// 新增文件下载路由处理
func DownloadFile(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	versionStr := c.DefaultQuery("version", "latest")

	var (
		version  int
		fileName string
	)

	// 获取版本信息
	if versionStr == "latest" {
		var file models.File
		if err := database.DB.Where("id = ?", fileID).First(&file).Error; err != nil {
			handleDatabaseError(c, err)
			return
		}
		version = file.Version
		fileName = file.Name
	} else {
		v, err := strconv.Atoi(versionStr)
		if err != nil || v <= 0 {
			utils.ErrorResponse(c, http.StatusBadRequest, "无效的版本号")
			return
		}
		version = v

		var fileVersion models.FileVersion
		if err := database.DB.Where("id = ? AND version = ?", fileID, version).
			First(&fileVersion).Error; err != nil {
			handleDatabaseError(c, err)
			return
		}
		fileName = fileVersion.Name
	}

	// 获取文件流
	reader, err := fileStorage.GetFile(fileID, version)
	if err != nil {
		if os.IsNotExist(err) { // 需要导入 "os"
			utils.ErrorResponse(c, http.StatusNotFound, "文件内容不存在")
		} else {
			utils.ErrorResponse(c, http.StatusInternalServerError, "文件访问失败")
		}
		return
	}
	defer reader.Close()

	// 设置响应头（支持中文文件名）
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fileName))) // 需要导入 "net/url"

	// 传输文件内容
	if _, err := io.Copy(c.Writer, reader); err != nil {
		log.Printf("文件传输中断: %v", err)
	}
}

// handleDatabaseError 统一处理数据库错误
func handleDatabaseError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.ErrorResponse(c, http.StatusNotFound, "Record not found")
	} else {
		log.Printf("Database error: %v", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Database error")
	}
}

// RenameFile 处理文件重命名
func RenameFile(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	if fileID == "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := database.DB.Model(&models.File{}).
		Where("id = ?", fileID).
		Update("name", req.Name).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to update filename")
		return
	}

	utils.SuccessResponse(c, nil)
}

// ListVersions 列出文件版本
func ListVersions(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var versions []models.FileVersion
	if err := database.DB.Where("id = ?", fileID).
		Order("version DESC").
		Offset(offset).
		Limit(limit).
		Find(&versions).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}

	utils.SuccessResponse(c, versions)
}

// GetVersion 获取特定版本
func GetVersion(c *gin.Context) {
	fileID := utils.SanitizeID(c.Param("file_id"))
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid version number")
		return
	}

	var versionData models.FileVersion
	if err := database.DB.Where("id = ? AND version = ?", fileID, version).
		First(&versionData).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.ErrorResponse(c, http.StatusNotFound, "Version not found")
		} else {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	utils.SuccessResponse(c, versionData)
}
