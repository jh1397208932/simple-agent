package httputil

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type SSEEvent struct {
	Event string
	Data  string
}

// SSEWriter 用于向SSE连接写入事件
type SSEWriter struct {
	c *gin.Context
}

// NewSSEWriter 创建新的SSE写入器
func NewSSEWriter(c *gin.Context) *SSEWriter {
	// 设置SSE相关头信息
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Flush()

	return &SSEWriter{c: c}
}

// WriteEvent 写入一个SSE事件
func (w *SSEWriter) WriteEvent(eventName, data string) error {
	var event SSEEvent = SSEEvent{Event: eventName, Data: data}
	// 格式化SSE事件
	var sb strings.Builder
	if event.Event != "" {
		sb.WriteString(fmt.Sprintf("event: %s\n", event.Event))
	}
	sb.WriteString(fmt.Sprintf("data: %s\n\n", event.Data))

	// 写入响应
	_, err := w.c.Writer.WriteString(sb.String())
	if err != nil {
		return err
	}

	// 刷新缓冲区以确保数据立即发送
	w.c.Writer.Flush()
	return nil
}
