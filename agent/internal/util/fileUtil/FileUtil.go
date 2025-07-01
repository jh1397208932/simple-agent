package fileUtil

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jhUtil/simple-agent-go/internal/util/errorUtil"

	"github.com/jhUtil/simple-agent-go/internal/util/randUtil"
)

// 上传文件并且替换目标文件 (会生成一个临时文件,然后将临时文件移动到目标路径下)
func InputFileToReplacePath(inputFile io.Reader, fileName string, toPath string) error {
	//使用uuid生成临时文件名
	sortId := randUtil.GenShortID()
	tmpFilePath, err := CreateTempFile(GenTmpNextLevelDirPath(sortId), sortId+".tmp")
	if err != nil {
		return errorUtil.NewF("上传存储临时文件创建失败,错误消息为: %s", err)
	}
	defer os.Remove(filepath.Clean(filepath.Dir(tmpFilePath) + "/")) // 删除临时文件夹
	tmpFile, err := os.OpenFile(tmpFilePath, os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return errorUtil.NewF("打开临时文件失败,错误消息为: %s", err)
	}
	defer tmpFile.Close()
	//将上传的文件内容写入临时文件
	_, err = io.Copy(tmpFile, inputFile)
	if err != nil {
		return errorUtil.NewF("上传文件内容写入临时文件失败,错误消息为: %s", err)
	}
	//先判断原始路径指向文件夹还是文件
	toPathIsDir, _, err := PathIsDir(toPath)
	log.Printf("isDir: %s %t", toPath, toPathIsDir)
	if err != nil {
		return errorUtil.NewF("检查目标路径是否为目录失败,错误消息为: %s", err)
	}
	var toPathAll string
	if toPathIsDir {
		//目标路径是一个目录  则拼接为完整文件名
		toPathAll = filepath.Join(toPath, fileName)
	} else {
		//不为目录 则直接使用目标路径
		toPathAll = toPath
	}
	//检查目标路径是否存在
	toPathAllExists, err := CheckPathExists(toPathAll)
	if err != nil {
		return errorUtil.NewF("检查目标路径下文件是否存在失败,错误消息为: %s", err)
	}
	var tmp2FilePath string
	if toPathAllExists {
		//路径存在  判断这个是一个目录还是文件
		pathExistIsDir, err := PathExistIsDir(toPathAll)
		if err != nil {
			return errorUtil.NewF("检查目标路径下文件是否是目录失败,错误消息为: %s", err)
		}
		if pathExistIsDir {
			//目标路径下文件是目录,不能上传文件到目录
			return errorUtil.NewF("目标路径文件是目录,不能上传文件到目录:%", toPathAll)
		} else {
			//目标目录下是一个文件, 移动目标文件到临时文件目录下
			tmp2FilePath = filepath.Dir(tmpFilePath) + "/" + filepath.Base(toPathAll)
			err = MoveFile(toPathAll, tmp2FilePath)
			if err != nil {
				return errorUtil.NewF("移动目标文件到临时文件目录失败,错误消息为: %s", err)
			}
		}

	}
	//在将临时文件移动到目标路径下
	err = MoveFile(tmpFilePath, toPathAll)
	if err != nil {
		//移动失败,则还原临时文件(原子化操作)
		if tmp2FilePath != "" {
			//先删除目标位置临时文件
			existsToPathAll, err := CheckPathExists(toPathAll)
			toPathAllPathIsDir, _, err2 := PathIsDir(toPathAll)
			if err != nil && err2 != nil && existsToPathAll && !toPathAllPathIsDir {
				os.Remove(filepath.Clean(toPathAll))
			}

			//如果临时文件路径不为空,则说明之前移动了目标文件到临时文件目录下
			err2 = MoveFile(tmp2FilePath, toPathAll)
			if err2 != nil {
				log.Printf("还原目标文件到原始位置失败,错误消息为: %s", err2)
			}
		}
		return errorUtil.NewF("移动目标文件到临时文件目录失败,错误消息为: %s", err)
	}

	return nil
}

func MoveFile(source string, target string) error {
	var err error
	//先判断目标路径是否是目录
	targetPathIsDir, _, err := PathIsDir(target)
	if err != nil {
		log.Printf("检查目标路径是否为目录时出错: %v", err)
		return err
	}
	//整理路径 去除没用的部分
	source = filepath.Clean(source)
	target = filepath.Clean(target)
	log.Printf("准备移动文件: 源路径: %s, 目标路径: %s", source, target)
	// 检查源文件是否为绝对路径
	if !filepath.IsAbs(source) {
		log.Println("源路径不是绝对路径")
		return errorUtil.New("路径必须为绝对路径")
	}
	// 检查目标路径是否为绝对路径
	if !filepath.IsAbs(target) {
		return errorUtil.New("目标路径必须为绝对路径")
	}
	// 检查源文件是否存在

	sourceExits, err := CheckPathExists(source)
	if err != nil {
		return err
	}
	if !sourceExits {
		return errorUtil.New("源文件不存在")
	}

	//检查目标文件或者目录是否存在  自动创建对应目录
	targetExits, err := CheckPathExists(target)
	if err != nil {
		return err
	}
	if targetPathIsDir {
		log.Println("目标路径是目录")
		targetDir := target
		log.Printf("最终路径为: %s", target)
		target = filepath.Join(target, filepath.Base(source))
		if !targetExits {
			//路径特征为文件夹 并且不存在 自动创建
			os.MkdirAll(targetDir, 0777)
		} else {
			//路经文件夹已经存在
			//判断目标目录+文件 是否已经存在
			targetJoinFileExits, err := CheckPathExists(target)
			if err != nil {
				return err
			}
			if targetJoinFileExits {
				return errorUtil.New("目标文件已存在")
			}
		}

	} else {
		log.Println("目标路径是文件")
		if targetExits {
			return errorUtil.New("目标文件已存在")
		}
		// 如果目标路径的父目录不存在，则创建它
		targetDir := filepath.Dir(target)
		targetDirExits, err := CheckPathExists(targetDir)
		if err != nil {
			return err
		}
		if !targetDirExits {
			if err := os.MkdirAll(targetDir, 0777); err != nil {
				return errorUtil.New("无法创建目标路径的父目录")
			}
		}
	}

	err = RenameOrCopy(source, target)
	if err != nil {
		log.Printf("移动文件失败: %v", err)
		return errorUtil.NewF("移动文件失败: %w", err)
	}
	log.Printf("文件移动成功: 源路径: %s, 目标路径: %s", source, target)
	// 返回成功
	return nil
}

// 跨文件系统安全的移动
func RenameOrCopy(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// 处理跨文件系统错误
	if !isCrossDeviceError(err) {
		return err
	}

	// 复制+删除源文件
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile 安全复制文件，当目标文件存在时不覆盖
func copyFile(src, dst string) error {
	// 检查目标文件是否存在
	if _, err := os.Stat(dst); err == nil {
		return errorUtil.NewF("目标文件已存在: %s", dst)
	} else if !os.IsNotExist(err) {
		return errorUtil.NewF("无法检查目标文件状态: %w", err)
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return errorUtil.NewF("无法打开源文件: %w", err)
	}
	defer srcFile.Close()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return errorUtil.NewF("无法获取源文件信息: %w", err)
	}

	// 创建目标文件（使用 O_EXCL 标志确保文件不存在）
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, srcInfo.Mode())
	if err != nil {
		return errorUtil.NewF("无法创建目标文件: %w", err)
	}
	defer dstFile.Close()

	// 复制文件内容
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		// 复制失败时清理部分写入的文件
		dstFile.Close()
		os.Remove(dst)
		return errorUtil.NewF("文件复制失败: %w", err)
	}

	// 同步写入确保数据落盘
	if err := dstFile.Sync(); err != nil {
		dstFile.Close()
		return errorUtil.NewF("文件同步失败: %w", err)
	}

	// 关闭目标文件
	if err := dstFile.Close(); err != nil {
		return errorUtil.NewF("关闭目标文件失败: %w", err)
	}

	// 保留修改时间
	mtime := srcInfo.ModTime()
	if err := os.Chtimes(dst, time.Now(), mtime); err != nil {
		log.Printf("警告：无法设置文件时间属性: %v", err)
	}

	return nil
}

// 检查是否跨设备错误
func isCrossDeviceError(err error) bool {
	if linkErr, ok := err.(*os.LinkError); ok {
		return linkErr.Err == syscall.EXDEV
	}
	return false
}

// 检查路径是否存在
// CheckPathExists 检查路径是否存在，如果路径为空则返回 false
// 如果路径不存在则返回 false，其他错误则返回 error
// 如果路径存在则返回 true
// 注意：此函数不会创建路径，只是检查路径是否存在
func CheckPathExists(path string) (bool, error) {
	// 检查路径是否为空
	if len(path) == 0 {
		return false, nil
	}
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 路径不存在
			return false, nil
		}
		// 其他错误
		log.Printf("无法访问路径 %s: %v", path, err)
		return false, errorUtil.New("无法访问路径: " + path)
	}
	// 路径存在
	return true, nil
}

// pathIsDir 判断路径是否以目录分隔符结尾（跨平台） 返回 检查路径是否为文件夹 2 路经是否存在 3错误
func PathIsDir(path string) (bool, bool, error) {
	// 检查路径是否为空
	if len(path) == 0 {
		return false, false, nil
	}
	stat, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		//路经不存在 根据路径结尾  判断想要目录还是文件
		lastChar := path[len(path)-1]
		return lastChar == '/' || lastChar == '\\', false, nil
	} else if err != nil {
		//不是文件不存在, 可能是权限问题或其他错误
		log.Printf("无法访问路径 %s: %v", path, err)
		return false, false, errorUtil.New("无法访问路径: " + path)
	} else {
		//如果路径存在 则判断是否为目录
		return stat.IsDir(), true, nil
	}

}

func PathExistIsDir(path string) (bool, error) {
	// 检查路径是否为空
	if len(path) == 0 {
		return false, nil
	}
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 路径不存在
			return false, nil
		}
		// 其他错误
		log.Printf("无法访问路径 %s: %v", path, err)
		return false, errorUtil.New("无法访问路径: " + path)
	}
	// 路径存在，返回是否为目录
	return stat.IsDir(), nil
}
