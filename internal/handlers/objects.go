package handlers

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "weboffice/internal/config"
    "weboffice/internal/database"
    "weboffice/internal/models"
    "weboffice/internal/utils"
)

// UploadObject 处理附件上传
func UploadObject(c *gin.Context) {
    key := c.Param("key")
    data, err := c.GetRawData()
    if err != nil {
        utils.ErrorResponse(c, http.StatusBadRequest, "Failed to read object data")
        return
    }

    hash := md5.Sum(data)
    digest := hex.EncodeToString(hash[:])

    attachment := models.Attachment{
        Key:       key,
        Data:      data,
        CreatedAt: time.Now().Unix(),
    }

    if err := database.DB.Create(&attachment).Error; err != nil {
        utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to store object")
        return
    }

    utils.SuccessResponse(c, gin.H{
        "digest": digest,
    })
}

// GetObjectURL 处理获取附件URL
func GetObjectURL(c *gin.Context) {
    key := c.Param("key")
    cfg := config.LoadConfig()

    utils.SuccessResponse(c, gin.H{
        "url": fmt.Sprintf("%s/objects/%s", cfg.StorageURL, key),
    })
}

// CopyObject 处理对象复制
func CopyObject(c *gin.Context) {
    var req struct {
        KeyDict map[string]string `json:"key_dict"`  // 修复结构体标签语法
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request body")
        return
    }

    err := database.DB.Transaction(func(tx *gorm.DB) error {
        for srcKey, dstKey := range req.KeyDict {
            var src models.Attachment
            if err := tx.Where("key = ?", srcKey).First(&src).Error; err != nil {
                return fmt.Errorf("source object %s not found", srcKey)
            }

            dst := models.Attachment{
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
        utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to copy objects: "+err.Error())
        return
    }

    utils.SuccessResponse(c, nil)
}