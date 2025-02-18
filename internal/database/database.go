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
)

var DB *gorm.DB

// InitDB函数用于初始化数据库连接
func InitDB(cfg *config.DBConfig) error {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
        PrepareStmt: true,
    })

    if err!= nil {
        // 在数据库连接失败时使用log包记录错误信息
        log.Printf("数据库连接失败: %v", err)
        return fmt.Errorf("database connection failed: %w", err)
    }

    sqlDB, _ := db.DB()
    sqlDB.SetMaxIdleConns(10)
    sqlDB.SetMaxOpenConns(100)
    sqlDB.SetConnMaxLifetime(time.Hour)

    if err := autoMigrate(db); err!= nil {
        // 在数据库迁移失败时使用log包记录错误信息
        log.Printf("数据库迁移失败: %v", err)
        return fmt.Errorf("database migration failed: %w", err)
    }

    DB = db
    return nil
}

// autoMigrate函数用于自动执行数据库迁移
func autoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.File{},
        &models.FileVersion{},
        &models.User{},
        &models.Watermark{},
        &models.Attachment{},
    )
}

// InitTestData函数用于初始化测试数据
func InitTestData() error {
    return DB.Transaction(func(tx *gorm.DB) error {
        // 清理旧测试数据
        if err := tx.Exec("DELETE FROM users WHERE id =?", "user1").Error; err!= nil {
            // 在清理用户数据失败时使用log包记录错误信息
            log.Printf("清理用户数据失败: %v", err)
            return fmt.Errorf("清理用户数据失败: %w", err)
        }

        if err := tx.Exec("DELETE FROM files WHERE id =?", "file123").Error; err!= nil {
            // 在清理文件数据失败时使用log包记录错误信息
            log.Printf("清理文件数据失败: %v", err)
            return fmt.Errorf("清理文件数据失败: %w", err)
        }

        if err := tx.Exec("DELETE FROM watermarks WHERE file_id =?", "file123").Error; err!= nil {
            // 在清理水印数据失败时使用log包记录错误信息
            log.Printf("清理水印数据失败: %v", err)
            return fmt.Errorf("清理水印数据失败: %w", err)
        }

        // 修复点：使用反引号转义保留字key
        if err := tx.Exec("DELETE FROM attachments WHERE `key` =?", "sample_key").Error; err!= nil {
            // 在清理附件数据失败时使用log包记录错误信息
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
        }).Create(&user).Error; err!= nil {
            // 在初始化用户失败时使用log包记录错误信息
            log.Printf("初始化用户失败: %v", err)
            return fmt.Errorf("初始化用户失败: %w", err)
        }

        // 初始化主文件
        now := time.Now().Unix()
        file := models.File{
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
            UpdateAll: true,
        }).Create(&file).Error; err!= nil {
            // 在初始化文件失败时使用log包记录错误信息
            log.Printf("初始化文件失败: %v", err)
            return fmt.Errorf("初始化文件失败: %w", err)
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
            FirstOrCreate(&watermark).Error; err!= nil {
            // 在初始化水印失败时使用log包记录错误信息
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
        }).Create(&attachment).Error; err!= nil {
            // 在初始化附件失败时使用log包记录错误信息
            log.Printf("初始化附件失败: %v", err)
            return fmt.Errorf("初始化附件失败: %w", err)
        }

        return nil
    })
}