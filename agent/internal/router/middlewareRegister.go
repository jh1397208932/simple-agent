package router

import (
	"github.com/gin-gonic/gin"
	"github.com/jhUtil/simple-agent-go/internal/router/middleware"
)

// 初始化插件
func InitMiddlewareRegister(r *gin.Engine, param map[string]string) {
	//认证
	r.Use(middleware.AuthMiddleware(param["aesKey"]))
}
