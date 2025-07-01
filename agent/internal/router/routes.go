package router

import (
	"github.com/gin-gonic/gin"
	v1Api "github.com/jhUtil/simple-agent-go/internal/api/v1"
)

// 初始化路由组
func InitRouter(r *gin.Engine) {
	iGroup := r.Group("/i")
	v1Api.InitRouterGroup(iGroup)
	// 其他路由组可以在这里继续添加

}
