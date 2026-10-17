package main

import (
	"log"
	"net/http"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/handler"
	"github.com/LennyFace24/MiniAgent/internal/middleware"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 运行时加载模型配置；格式错误不 panic，降级到内置默认并告警
	registry, err := modelmgr.Load("model.json")
	if err != nil {
		log.Printf("[警告] 加载模型配置失败，使用内置默认模型: %v", err)
		registry = modelmgr.DefaultRegistry()
	}

	defaultModel := registry.Default()
	contexttool.GetContextState().ConfigureMaxTokens(defaultModel.ContextBudgetTokens())

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

	handler.SetupHandler(r, registry)
	handler.SetupRoutes(r)

	r.Run(cfg.Server.Host + ":" + cfg.Server.Port)
}
