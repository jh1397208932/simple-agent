package cmdapi

import (
	"context"
	"sync"

	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jhUtil/simple-agent-go/internal/dto/result"
	"github.com/jhUtil/simple-agent-go/internal/util/cmdUtil"
	"github.com/jhUtil/simple-agent-go/internal/util/httputil"
)

// ExecuteCommandHandler 处理命令执行请求
func ExecuteCommandHandler(c *gin.Context) {
	// 从请求体中获取命令
	command := c.PostForm("command")
	log.Printf("执行命令为 : %s \n", command)
	if command == "" {
		c.JSON(http.StatusBadRequest, result.NewFailRespM("命令不能为空"))
		return
	}

	// 创建SSE写入器
	sseWriter := httputil.NewSSEWriter(c)

	// 创建命令执行器
	//executor := cmdUtil.NewCommandExecutor()

	// 创建上下文用于超时和取消控制
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	// 创建输出通道
	output := make(chan string, 100)
	// 心跳定时器
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	var once sync.Once

	// 启动命令执行
	go func() {
		defer cancel() // 确保命令执行完成后取消上下文
		cmdUtil.StreamCommand(ctx, command, output)
		// if err := ; err != nil {
		// 	log.Printf("命令执行错误: %v", err)
		// 	sseWriter.WriteEvent("error", fmt.Sprintf("命令执行错误: %v", err))
		// }

	}()
	defer log.Println("结束命令执行")
	// 监听上下文取消和输出通道
	for {

		select {
		case <-heartbeat.C:

			//心跳
			sseWriter.WriteEvent("heartbeat", "1")
		case <-ctx.Done():
			log.Printf("请求上下文结束,命令执行终止")
			// 上下文被取消，结束流
			sseWriter.WriteEvent("end", "命令执行已终止")
			once.Do(func() {
				heartbeat.Stop()
				close(output) // 仅关闭一次

			})
			return
		case line, ok := <-output:

			if ok {
				//log.Print("发送:" + line)
				// 发送输出行
				if err := sseWriter.WriteEvent("data", line); err != nil {
					log.Printf("写入SSE事件失败: %v", err)
					return
				}
			} else {
				log.Printf("命令执行终止")
				return
			}

		}
	}

}
