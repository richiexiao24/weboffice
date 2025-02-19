package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FileStorage struct {
	basePath string
}

func NewStorage(basePath string) *FileStorage {
	os.MkdirAll(basePath, 0755)
	return &FileStorage{basePath: basePath}
}

// 保存文件并返回存储路径// 修改SaveFile函数签名，增加fileName参数
func (s *FileStorage) SaveFile(fileID string, version int, fileName string, src io.Reader) error {
	// 生成带版本和文件名的路径
	versionDir := filepath.Join(s.basePath, fileID, fmt.Sprintf("v%d", version))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 使用原始文件名保存
	filePath := filepath.Join(versionDir, fileName)
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, src); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

// 获取文件读取流
func (s *FileStorage) GetFile(fileID string, version int) (io.ReadCloser, error) {
	filePath := filepath.Join(s.basePath, fileID, fmt.Sprintf("v%d", version), "content")
	return os.Open(filePath)
}
