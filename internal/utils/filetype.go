package utils

import (
	"fmt" // 新增导入
	"mime"
	"mime/multipart" // 新增导入
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string][]string{
	// 文字文档
	"application/msword": {"doc", "dot"},
	"application/vnd.ms-word.document.macroEnabled.12":                        {"docm"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {"docx"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.template": {"dotx"},
	"application/wps-office.wps":                                              {"wps"},
	"application/wps-office.wpt":                                              {"wpt"},
	"application/rtf":                                                         {"rtf"},
	"text/plain":                                                              {"txt"},
	"text/xml":                                                                {"xml"},
	"application/xhtml+xml":                                                   {"mhtml", "mht"},
	"text/html":                                                               {"html", "htm"},
	"application/uof":                                                         {"uof", "uot3"},

	// 表格文档
	"application/vnd.ms-excel": {"xls", "xlt"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":    {"xlsx"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.template": {"xltx"},
	"application/vnd.ms-excel.sheet.macroEnabled.12":                       {"xlsm"},
	"application/vnd.ms-excel.template.macroEnabled.12":                    {"xltm"},
	"text/csv":        {"csv"},
	"application/ett": {"ett"},

	// 演示文档
	"application/vnd.ms-powerpoint":                                             {"ppt", "pps", "pot"},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {"pptx"},
	"application/vnd.openxmlformats-officedocument.presentationml.template":     {"potx"},
	"application/vnd.openxmlformats-officedocument.presentationml.slideshow":    {"ppsx"},
	"application/vnd.ms-powerpoint.presentation.macroEnabled.12":                {"pptm"},
	"application/vnd.ms-powerpoint.template.macroEnabled.12":                    {"potm"},
	"application/vnd.ms-powerpoint.slideshow.macroEnabled.12":                   {"ppsm"},
	"application/dps": {"dps", "dpt"},
}

func init() {
	// 注册Office文档类型
	mime.AddExtensionType(".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	mime.AddExtensionType(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	mime.AddExtensionType(".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation")
}

// ValidateFileType 验证文件类型合法性
func ValidateFileType(fileHeader *multipart.FileHeader) error {
	// 获取MIME类型
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		// 通过扩展名推断类型
		ext := filepath.Ext(fileHeader.Filename)
		mimeType = mime.TypeByExtension(ext)
	}

	// 获取小写扩展名（不带点）
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileHeader.Filename), "."))

	// 双重校验机制
	for mimePattern, allowedExts := range allowedExtensions {
		// 允许主类型匹配（如 application/vnd.openxmlformats-officedocument.*）
		if strings.HasPrefix(mimeType, mimePattern) {
			for _, allowedExt := range allowedExts {
				if ext == allowedExt {
					return nil
				}
			}
		}
	}

	// 放宽校验逻辑：如果扩展名匹配但MIME类型未知也允许通过
	if mimeType == "application/octet-stream" {
		for _, allowedExts := range allowedExtensions {
			for _, allowedExt := range allowedExts {
				if ext == allowedExt {
					return nil
				}
			}
		}
	}

	// 未匹配到允许的类型
	return fmt.Errorf("不支持的文件类型: MIME类型=%s 扩展名=%s", mimeType, ext)
}
