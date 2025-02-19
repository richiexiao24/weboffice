package database

import (
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"weboffice/internal/config"
	"weboffice/internal/models"

	"bytes"                      // 新增
	"os"                         // 新增
	"weboffice/internal/storage" // 新增
)

var DB *gorm.DB

// InitDB函数用于初始化数据库连接
func InitDB(cfg *config.DBConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})

	if err != nil {
		// 在数据库连接失败时使用log包记录错误信息
		log.Printf("数据库连接失败: %v", err)
		return fmt.Errorf("database connection failed: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := autoMigrate(db); err != nil {
		// 在数据库迁移失败时使用log包记录错误信息
		log.Printf("数据库迁移失败: %v", err)
		return fmt.Errorf("database migration failed: %w", err)
	}

	DB = db
	return nil
}

// autoMigrate函数用于自动执行数据库迁移
func autoMigrate(db *gorm.DB) error {
	// 设置优化后的表选项
	return db.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC").AutoMigrate(
		&models.File{},
		&models.FileVersion{},
		&models.User{},
		&models.Watermark{},
		&models.Attachment{},
	)
}

func InitTestData() error {
	// 先执行数据库初始化
	if err := initDatabaseData(); err != nil {
		return err
	}

	// 再执行存储初始化
	return initFileStorageData()
}

// 数据库数据初始化（保持原有事务逻辑）
func initDatabaseData() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// 清理旧测试数据
		if err := tx.Exec("DELETE FROM users WHERE id =?", "user1").Error; err != nil {
			log.Printf("清理用户数据失败: %v", err)
			return fmt.Errorf("清理用户数据失败: %w", err)
		}

		// 清理所有可能残留的测试数据（通配符匹配）
		if err := tx.Exec("DELETE FROM files WHERE id LIKE 'file%'").Error; err != nil {
			return fmt.Errorf("清理文件数据失败: %w", err)
		}
		if err := tx.Exec("DELETE FROM file_versions WHERE id LIKE 'file%'").Error; err != nil {
			return fmt.Errorf("清理版本数据失败: %w", err)
		}

		if err := tx.Exec("DELETE FROM watermarks WHERE file_id =?", "file123").Error; err != nil {
			log.Printf("清理水印数据失败: %v", err)
			return fmt.Errorf("清理水印数据失败: %w", err)
		}

		if err := tx.Exec("DELETE FROM attachments WHERE `key` =?", "sample_key").Error; err != nil {
			log.Printf("清理附件数据失败: %v", err)
			return fmt.Errorf("清理附件数据失败: %w", err)
		}

		// 初始化用户
		user := models.User{
			ID:        "user1",
			Name:      "Admin",
			AvatarURL: "https://example.com/avatar.jpg",
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"name":       user.Name,
				"avatar_url": user.AvatarURL,
			}),
		}).Create(&user).Error; err != nil {
			log.Printf("初始化用户失败: %v", err)
			return fmt.Errorf("初始化用户失败: %w", err)
		}

		// 初始化主文件
		now := time.Now().Unix()
		file := models.File{
			ID:         "file123",
			Name:       "测试文档v1.docx",
			Version:    1, //明确初始化主文件的版本为1
			Size:       1024,
			CreateTime: now,
			ModifyTime: now,
			CreatorID:  "user1",
			ModifierID: "user1",
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(&file).Error; err != nil {
			log.Printf("初始化文件失败: %v", err)
			return fmt.Errorf("初始化文件失败: %w", err)
		}
		// 同步初始化版本记录（新增代码）
		fileVersion := models.FileVersion{
			ID:         file.ID,
			Version:    file.Version, // 使用与主文件相同的版本号
			Name:       file.Name,
			Size:       file.Size,
			CreateTime: file.CreateTime,
			ModifierID: file.ModifierID,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}, {Name: "version"}},
			DoNothing: true,
		}).Create(&fileVersion).Error; err != nil {
			log.Printf("初始化文件版本失败: %v", err)
			return fmt.Errorf("初始化文件版本失败: %w", err)
		}

		// 初始化水印
		watermark := models.Watermark{
			FileID:     "file123",
			Type:       1,
			Value:      "Confidential",
			Horizontal: 50,
			Vertical:   100,
		}
		if err := tx.Where(models.Watermark{FileID: "file123"}).
			Assign(watermark).
			FirstOrCreate(&watermark).Error; err != nil {
			log.Printf("初始化水印失败: %v", err)
			return fmt.Errorf("初始化水印失败: %w", err)
		}

		// 初始化附件
		attachment := models.Attachment{
			Key:       "sample_key",
			Data:      []byte("sample content"),
			CreatedAt: now,
		}
		if len(attachment.Data) == 0 {
			return errors.New("附件内容不能为空")
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "`key`"}},
			DoNothing: true,
		}).Create(&attachment).Error; err != nil {
			log.Printf("初始化附件失败: %v", err)
			return fmt.Errorf("初始化附件失败: %w", err)
		}

		return nil
	})
}

// 文件存储初始化（新增函数）
func initFileStorageData() error {
	cfg := config.LoadConfig()

	// 创建存储目录
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		log.Printf("创建存储目录失败: %v", err)
		return fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 初始化存储实例
	storage := storage.NewStorage(cfg.StoragePath)

	// 保存测试文件
	testContent := bytes.NewReader([]byte("测试文档内容"))
	if err := storage.SaveFile("file123", 1, "测试文档v1.docx", testContent); err != nil {
		log.Printf("存储测试文件失败: %v", err)
		return fmt.Errorf("存储测试文件失败: %w", err)
	}

	return nil
}

func Transaction(fc func(tx *gorm.DB) error) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// 设置事务隔离级别为可重复读
		if err := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ").Error; err != nil {
			return fmt.Errorf("设置事务隔离级别失败: %w", err)
		}
		return fc(tx)
	})
}
