package fileService

import (
	"io"

	"github.com/jhUtil/simple-agent-go/internal/util/errorUtil"
	"github.com/jhUtil/simple-agent-go/internal/util/fileUtil"
)

func MoveFile(source string, target string) error {
	err := fileUtil.MoveFile(source, target)
	return err
}

func UploadFileToReplacePath(inputFile io.Reader, fileName string, toPath string) error {
	if toPath == "" || fileName == "" {
		return errorUtil.NewF("目标路径或文件名不能为空")
	} else {
	}

	err := fileUtil.InputFileToReplacePath(inputFile, fileName, toPath)
	if err != nil {
		return err
	}
	return nil
}
