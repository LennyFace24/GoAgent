package handler

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// SetupFrontend serves the built frontend without intercepting API routes or
// falling back to HTML for unknown paths. distDir is relative to the working
// directory unless absolute; a missing index is returned as a filesystem error.
func SetupFrontend(r *gin.Engine, distDir string) error {
	root, err := filepath.Abs(distDir)
	if err != nil {
		return fmt.Errorf("解析前端构建目录失败: %w", err)
	}
	index, err := os.OpenInRoot(root, "index.html")
	if err != nil {
		return fmt.Errorf("读取前端构建产物失败，请先执行 npm --prefix frontend run build: %w", err)
	}
	info, err := index.Stat()
	index.Close()
	if err != nil {
		return fmt.Errorf("读取前端首页状态失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("前端 index.html 不是普通文件")
	}

	serveIndex := func(c *gin.Context) {
		serveFrontendFile(c, root, "index.html")
	}
	serveAsset := func(c *gin.Context) {
		name := strings.TrimPrefix(c.Param("filepath"), "/")
		if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
			c.Status(http.StatusNotFound)
			return
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				c.Status(http.StatusNotFound)
				return
			}
		}
		serveFrontendFile(c, root, "assets/"+name)
	}

	r.GET("/", serveIndex)
	r.HEAD("/", serveIndex)
	r.GET("/index.html", serveIndex)
	r.HEAD("/index.html", serveIndex)
	r.GET("/assets/*filepath", serveAsset)
	r.HEAD("/assets/*filepath", serveAsset)
	return nil
}

func serveFrontendFile(c *gin.Context, root, name string) {
	// OpenInRoot rejects traversal and symlinks escaping dist, including on
	// Windows. Opening a file rather than a FileServer also forbids listings.
	file, err := os.OpenInRoot(root, name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}
