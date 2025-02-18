package config

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Name     string
}

type AppConfig struct {
	DB         *DBConfig
	StorageURL string // 存储服务基础地址
	UploadURL  string // 上传服务端点
	ServerPort int
}

func LoadConfig() *AppConfig {
	return &AppConfig{
		DB: &DBConfig{
			User:     "webuser",
			Password: "Aa@123456",
			Host:     "localhost",
			Port:     3306,
			Name:     "weboffice",
		},
		StorageURL: "http://storage.example.com", // 新增配置项
		UploadURL:  "http://upload.example.com",  // 新增配置项
		ServerPort: 8080,
	}
}
