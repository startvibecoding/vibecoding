package vendored

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// rgData 和 fdData 由各平台的 embed_*.go 文件定义
// 通过 go:embed 嵌入对应的二进制数据

// binDir 返回 ~/.vibecoding/bin/ 目录路径
func binDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}
	return filepath.Join(home, ".vibecoding", "bin"), nil
}

// Ensure 确保 rg 和 fd 已解压到 ~/.vibecoding/bin/
// 首次运行时从嵌入数据写入，后续跳过
func Ensure() error {
	dir, err := binDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	// 写入 rg
	rgPath := filepath.Join(dir, "rg"+ext)
	if err := extractBinary(rgPath, rgData); err != nil {
		return fmt.Errorf("提取 rg 失败: %w", err)
	}

	// 写入 fd
	fdPath := filepath.Join(dir, "fd"+ext)
	if err := extractBinary(fdPath, fdData); err != nil {
		return fmt.Errorf("提取 fd 失败: %w", err)
	}

	return nil
}

// RgPath 返回 rg 二进制路径
func RgPath() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	dir, err := binDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "rg"+ext)
}

// FdPath 返回 fd 二进制路径
func FdPath() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	dir, err := binDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "fd"+ext)
}

// extractBinary 将嵌入的二进制数据写入目标路径
// 如果目标已存在且大小相同则跳过
func extractBinary(dest string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("嵌入数据为空（当前平台可能不受支持）")
	}

	// 检查是否已存在
	if info, err := os.Stat(dest); err == nil {
		if info.Size() == int64(len(data)) {
			return nil // 已存在且大小一致，跳过
		}
	}

	// 写入文件
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", dest, err)
	}

	return nil
}
