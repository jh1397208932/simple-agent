package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jhUtil/simple-agent-go/internal/router"
)

func main() {
	//设置调试模式
	gin.SetMode(gin.DebugMode)
	// 强制日志颜色化
	gin.ForceConsoleColor()

	g := gin.Default()
	g.Use(gin.Recovery())
	//router.Use(gin.Logger())
	// LoggerWithFormatter 中间件会写入日志到 gin.DefaultWriter
	// 默认 gin.DefaultWriter = os.Stdout
	// router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
	// 	// 你的自定义格式
	// 	return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
	// 		param.ClientIP,
	// 		param.TimeStamp.Format(time.RFC1123),
	// 		param.Method,
	// 		param.Path,
	// 		param.Request.Proto,
	// 		param.StatusCode,
	// 		param.Latency,
	// 		param.Request.UserAgent(),
	// 		param.ErrorMessage,
	// 	)
	// }))

	// router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
	// 	// 状态码颜色标记
	// 	statusColor := ""
	// 	switch {
	// 	case param.StatusCode >= 500:
	// 		statusColor = "\033[31m" // 红色表示服务端错误
	// 	case param.StatusCode >= 400:
	// 		statusColor = "\033[33m" // 黄色表示客户端错误
	// 	case param.StatusCode >= 300:
	// 		statusColor = "\033[36m" // 青色表示重定向
	// 	default:
	// 		statusColor = "\033[32m" // 绿色表示成功
	// 	}

	// 	return fmt.Sprintf(
	// 		"[GIN] %s | %s %3d %s | %13v | %15s | %-7s %s\n\033[0m",
	// 		param.TimeStamp.Format("2006/01/02 - 15:04:05"), // 中文日期格式
	// 		statusColor, param.StatusCode, statusColor, // 彩色状态码
	// 		param.Latency,  // 请求耗时
	// 		param.ClientIP, // 客户端IP
	// 		param.Method,   // 请求方法
	// 		param.Path,     // 请求路径
	// 	)
	// }))
	router.InitRouter(g) // 初始化路由组
	// router.GET("/health", func(c *gin.Context) {
	// 	c.JSON(http.StatusOK, gin.H{
	// 		"status": "healthy",
	// 	})
	// })
	g.Run(":31000") // 监听并在 8080 端口上服务
	fmt.Println("server run 31000")
}
