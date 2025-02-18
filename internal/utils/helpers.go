package utils

import "strings"

// SanitizeID函数用于清理输入的ID参数
func SanitizeID(id string) string {
    return strings.TrimSpace(id)
}