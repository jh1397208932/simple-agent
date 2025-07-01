package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/jhUtil/simple-agent-go/internal/api/v1/cmdapi"
	"github.com/jhUtil/simple-agent-go/internal/api/v1/fileApi"
)

func InitRouterGroup(g *gin.RouterGroup) {
	//文件
	fileGroup := g.Group("/file")
	{
		// 移动文件
		// POST /i/file/moveFile
		// 请求体: {"source": "源文件路径", "target": "目标文件路径"}
		// 响应: 成功或失败消息
		// 示例: {"message": "文件移动成功"}
		// 注意: 确保源文件路径和目标文件路径是有效的
		// 注意: 需要处理文件移动的权限和错误情况
		// 注意: 需要确保目标路径存在，如果不存在则需要创建
		// 注意: 如果目标路径是一个目录，则文件将被移动到该目录
		//curl -X POST -H "Content-Type: application/json" -d '{"source": "/path/to/source/file.txt", "target": "/path/to/target/file.txt"}' http://localhost:8080/i/file/moveFile
		fileGroup.POST("/moveFile", fileApi.MoveFile)
		// 文件上传
		// POST /i/file/uploadToPathReplace
		// 请求体: form-data 包含文件字段 "file" 和目标路径字段 "toFilePath"
		// 响应: 成功或失败消息
		// 示例: {"message": "文件上传成功"}
		// 注意: 确保目标路径是有效的，并且有权限写
		//curl -X POST -F "file=@/path/to/your/file.txt" -F "toFilePath=/path/to/target/file.txt" http://localhost:8080/i/file/uploadToPathReplace
		fileGroup.POST("/uploadToPathReplace", fileApi.UploadHandler)
	}
	//命令执行 (未格式化)
	cmdGroup := g.Group("cmd")
	{
		//执行命令
		//POST /i/cmd/executeCmdSse
		//curl -X POST -F "command=命令"  http://localhost:8080/i/cmd/executeCmdSse
		cmdGroup.POST("executeCmdSse", cmdapi.ExecuteCommandHandler)
	}
}
