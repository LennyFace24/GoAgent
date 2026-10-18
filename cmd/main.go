package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return err
	}

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

	startupCtx, cancelStartup := context.WithTimeout(ctx, 30*time.Second)
	mcpManager, err := handler.SetupHandler(startupCtx, registry)
	cancelStartup()
	if err != nil {
		return fmt.Errorf("初始化服务失败: %w", err)
	}
	defer func() {
		if err := mcpManager.Close(); err != nil {
			log.Printf("关闭 MCP 连接失败: %v", err)
		}
	}()
	handler.SetupRoutes(r)
	if err := handler.SetupFrontend(r, "frontend/dist"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("初始化前端托管失败: %w", err)
		}
		log.Printf("前端未托管，仅启动 API；需要页面时先执行 npm --prefix frontend run build: %v", err)
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.Server.Host, cfg.Server.Port))
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:     r,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	log.Printf("HTTP 服务监听 %s", listener.Addr())
	return serve(ctx, server, listener)
}

// serve drains HTTP requests before run releases the MCP sessions.
func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		if listenErr := <-serveErr; !errors.Is(listenErr, http.ErrServerClosed) {
			err = errors.Join(err, listenErr)
		}
		return err
	}
}
