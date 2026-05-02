package main

import (
	// gin 框架
	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/handler"
	"github.com/LennyFace24/MiniAgent/internal/middleware"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig("config.yaml")
	if cfg == nil {
		panic("加载配置文件失败")
	}

	r := gin.Default()

	store := cookie.NewStore([]byte("session-secret"))
	r.Use(sessions.Sessions("goagent_session", store))
	r.Use(middleware.SessionIDMiddleware())

	handler.SetupHandler(r)
	handler.SetupRoutes(r)

	r.Run(cfg.Server.Host + ":" + cfg.Server.Port)
}
