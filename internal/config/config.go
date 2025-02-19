package config

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Name     string
}

type AppConfig struct {
	DB               *DBConfig
	StorageURL       string // 存储服务基础地址
	UploadURL        string // 上传服务端点
	ServerPort       int
	StoragePath      string              // 新增本地存储路径配置
	AllowedFileTypes map[string][]string `yaml:"allowed_file_types"`
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
		StorageURL:  "http://storage.example.com", // 新增配置项
		UploadURL:   "http://upload.example.com",  // 新增配置项
		ServerPort:  8080,
		StoragePath: "./storage", // 本地存储根目录
		AllowedFileTypes: map[string][]string{
			"document": {
				"doc", "dot", "wps", "wpt", "docx", "dotx", "docm", "dotm",
				"rtf", "txt", "xml", "mhtml", "mht", "html", "htm", "uof", "uot3",
			},
			"spreadsheet": {
				"xls", "xlt", "et", "xlsx", "xltx", "csv", "xlsm", "xltm", "ett",
			},
			"presentation": {
				"ppt", "pptx", "pptm", "ppsx", "ppsm", "pps", "potx", "potm", "dpt", "dps", "pot",
			},
		},
	}
}
