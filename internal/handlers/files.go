package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	"strconv"
   

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"weboffice/internal/config" // 新增导入
	"weboffice/internal/database"
	"weboffice/internal/models"
	"weboffice/internal/utils"
)

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
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid file ID")
		return
	}

	var req struct {
		Request  map[string]interface{} `json:"request"`
		Response map[string]interface{} `json:"response"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var file models.File
		if err := tx.Where("id = ?", fileID).First(&file).Error; err != nil {
			return err
		}

		newVersion := models.FileVersion{
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

		updateFields := map[string]interface{}{
			"version":     newVersion.Version,
			"modify_time": time.Now().Unix(),
			"size":        req.Request["size"],
		}

		return tx.Model(&models.File{}).Where("id = ?", fileID).Updates(updateFields).Error
	})

	if err != nil {
		log.Printf("Upload complete failed: %v", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to process upload")
		return
	}

	utils.SuccessResponse(c, nil)
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
