package main

import (
	"net/http"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/handler"
	"github.com/LennyFace24/MiniAgent/internal/middleware"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig("config.yaml")
	if cfg == nil {
		panic("加载配置文件失败")
	}
	if cfg.Session.SecretKey == "" {
		panic("session.secret_key 不能为空")
	}

	contexttool.GetContextState().ConfigureMaxTokens(cfg.ContextBudgetTokens())

	r := gin.Default()
	r.Use(middleware.CORS())

	store := cookie.NewStore([]byte(cfg.Session.SecretKey))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(sessions.Sessions("goagent_session", store))
	r.Use(middleware.SessionIDMiddleware())

	handler.SetupHandler(r)
	handler.SetupRoutes(r)

	r.Run(cfg.Server.Host + ":" + cfg.Server.Port)
}
