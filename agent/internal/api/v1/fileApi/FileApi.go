package fileApi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jhUtil/simple-agent-go/internal/dto/param"
	"github.com/jhUtil/simple-agent-go/internal/dto/result"
	"github.com/jhUtil/simple-agent-go/internal/service/fileService"
)

// 移动文件
func MoveFile(c *gin.Context) {
	var param param.MoveFileParam
	// 调用服务层的 MoveFile 方法
	c.BindJSON(&param)
	err := fileService.MoveFile(param.Source, param.Target)
	if err != nil {
		fmt.Println("移动文件失败:", err)
		c.JSON(http.StatusOK, result.NewFailRespM(err.Error()))
		return
	} else {
		fmt.Println("移动文件:", param.Source, "到", param.Target)
		c.JSON(http.StatusOK, result.NewSuccessResp())
	}

}

// 文件上传处理函数
func UploadHandler(c *gin.Context) {
	request := c.Request
	file, fileHeader, err := request.FormFile("file") // 获取表单中的文件字段
	if err != nil {
		c.JSON(200, result.NewFailRespM("文件上传失败: "+err.Error()))
		return
	}
	toFilePath := request.FormValue("toFilePath") // 获取表单中的 toFilePath 字段

	err = fileService.UploadFileToReplacePath(file, fileHeader.Filename, toFilePath) // 假设上传到 /tmp/upload/ 目录
	if err != nil {
		c.JSON(200, result.NewFailRespM("文件上传失败: "+err.Error()))
		return
	} else {
		c.JSON(200, result.NewSuccessResp())
		return
	}
}
