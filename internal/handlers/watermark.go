package handlers

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "weboffice/internal/database"
    "weboffice/internal/models"
    "weboffice/internal/utils"
)

// GetWatermark 处理获取水印配置
func GetWatermark(c *gin.Context) {
    fileID := utils.SanitizeID(c.Param("file_id"))
    if fileID == "" {
        utils.ErrorResponse(c, http.StatusBadRequest, "Invalid file ID")
        return
    }

    var watermark models.Watermark
    err := database.DB.Where("file_id = ?", fileID).First(&watermark).Error

    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // 未找到水印时返回默认类型
            utils.SuccessResponse(c, gin.H{"type": 0})
        } else {
            utils.ErrorResponse(c, http.StatusInternalServerError, "Database error")
        }
        return
    }

    utils.SuccessResponse(c, watermark)
}