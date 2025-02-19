package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"weboffice/internal/config"
	"weboffice/internal/database"
	"weboffice/internal/handlers"
	"weboffice/internal/routes"
	"weboffice/internal/storage"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()

	// 初始化存储系统（正确方式）
	fileStorage := storage.NewStorage(cfg.StoragePath)
	handlers.InitFileStorage(fileStorage) // 传递存储实例而非配置

	// 初始化数据库
	if err := database.InitDB(cfg.DB); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}

	// 初始化测试数据
	if err := database.InitTestData(); err != nil {
		log.Fatalf("Test data initialization failed: %v", err)
	}

	// 创建Gin实例
	r := gin.Default()
	r.MaxMultipartMemory = 256 << 20 // 256MB内存缓冲，超过部分写入临时文件
	configureLogger(r)

	// 注册路由
	routes.RegisterRoutes(r)

	// 启动服务
	log.Printf("Starting server on :%d", cfg.ServerPort)
	if err := r.Run(fmt.Sprintf(":%d", cfg.ServerPort)); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}

func configureLogger(r *gin.Engine) {
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.ErrorMessage,
		)
	}))
}
