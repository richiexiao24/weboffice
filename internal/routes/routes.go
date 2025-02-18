package routes

import (
    "github.com/gin-gonic/gin"

    "weboffice/internal/handlers"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(r *gin.Engine) {
    // 文件相关路由
    fileGroup := r.Group("/v3/3rd/files")
    {
        fileGroup.GET("/:file_id", handlers.GetFile)
        fileGroup.GET("/:file_id/download", handlers.GetDownloadURL)
        fileGroup.GET("/:file_id/permission", handlers.GetPermissions)

        // 文件操作路由
        fileGroup.PUT("/:file_id/name", handlers.RenameFile)
        fileGroup.GET("/:file_id/versions", handlers.ListVersions)
        fileGroup.GET("/:file_id/versions/:version", handlers.GetVersion)
        fileGroup.GET("/:file_id/versions/:version/download", handlers.GetDownloadURL)

        // 上传相关路由
        fileGroup.GET("/:file_id/upload/prepare", handlers.PrepareUpload)
        fileGroup.POST("/:file_id/upload/address", handlers.GetUploadAddress)
        fileGroup.POST("/:file_id/upload/complete", handlers.UploadComplete)

        // 水印配置
        fileGroup.GET("/:file_id/watermark", handlers.GetWatermark)
    }

    	// 用户相关路由
    	userGroup := r.Group("/v3/3rd/users")
    {
        userGroup.GET("", handlers.GetUsers)
    }

    // 对象存储路由
    objectGroup := r.Group("/v3/3rd/object")
    {
        objectGroup.PUT("/:key", handlers.UploadObject)
        objectGroup.GET("/:key/url", handlers.GetObjectURL)
        objectGroup.POST("/copy", handlers.CopyObject)
    }
}