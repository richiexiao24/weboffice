package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"weboffice/internal/database"
	"weboffice/internal/models"
	"weboffice/internal/utils"
)

// GetUsers 处理获取用户信息的请求
func GetUsers(c *gin.Context) {
	userIDs := c.QueryArray("user_ids")
	if len(userIDs) == 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Missing user_ids parameter")
		return
	}

	var users []models.User
	if err := database.DB.Where("id IN (?)", userIDs).Find(&users).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}

	utils.SuccessResponse(c, users)
}
