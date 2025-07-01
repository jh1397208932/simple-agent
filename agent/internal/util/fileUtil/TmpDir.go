package fileUtil

import (
	"os"
	"path/filepath"

	"github.com/jhUtil/simple-agent-go/internal/util/errorUtil"
)

// 配置常量
const (
	TempFilePrefix = "j-simple-agent"
	TmpBaseDir     = "tmp"
)

// 创建临时文件
func CreateTempFile(dir, name string) (string, error) {
	// 获取系统临时目录 +程序统一目录 +业务目录
	tempDir := filepath.Join(os.TempDir(), TempFilePrefix, dir)
	os.MkdirAll(tempDir, 0755)
	// 创建临时文件
	tempFile, err := os.CreateTemp(tempDir, name)
	if err != nil {
		return "", errorUtil.NewF("创建临时文件失败: %s", err)
	}
	defer tempFile.Close()

	// 获取完整路径
	tempPath, err := filepath.Abs(tempFile.Name())
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", errorUtil.NewF("获取临时文件路径失败: %s", err)
	}

	return tempPath, nil
}

// 按本次业务 生成一个临时文件夹
func GenTmpNextLevelDirPath(dirName string) string {
	return TmpBaseDir + "/" + dirName + "/"
}
