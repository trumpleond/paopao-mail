package main

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"paopao-api/internal/config"
	"paopao-api/internal/db"
	"paopao-api/internal/handler"
	"paopao-api/internal/middleware"
	"paopao-api/internal/service"
	"paopao-api/internal/store"
	"paopao-api/web"
)

// Injected by -ldflags at build time (CI / release).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database failed", "err", err, "path", cfg.DBPath)
		os.Exit(1)
	}
	defer database.Close()

	accountStore := store.NewAccountStore(database)
	emailSvc := service.NewEmailService(cfg.UpstreamBase, cfg.UpstreamTimeoutSec, accountStore)

	accountH := handler.NewAccountHandler(accountStore)
	emailH := handler.NewEmailHandler(emailSvc)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLog())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"time":    time.Now().UTC(),
			"version": version,
			"commit":  commit,
		})
	})

	api := r.Group("/api")
	api.Use(middleware.APIKey(cfg.APIKey))
	{
		api.POST("/accounts/import", accountH.Import)
		api.POST("/accounts/pick", accountH.Pick)
		api.POST("/accounts/mark", accountH.MarkByEmail)
		api.POST("/accounts/:id/mark", accountH.Mark)
		api.POST("/accounts/:id/unmark", accountH.Unmark)
		api.GET("/accounts", accountH.List)
		api.GET("/accounts/:id", accountH.Get)
		api.PATCH("/accounts/:id", accountH.Update)
		api.DELETE("/accounts/:id", accountH.Delete)
		api.GET("/stats", accountH.Stats)
		api.GET("/emails", emailH.GetEmails)
	}

	mountWebUI(r)

	// Optional convenience: serve ./mail.txt for one-click import on the UI
	r.GET("/mail.txt", func(c *gin.Context) {
		for _, p := range []string{"mail.txt", "./mail.txt"} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				c.File(p)
				return
			}
		}
		c.Status(http.StatusNotFound)
	})

	uiHint := cfg.Addr
	if strings.HasPrefix(uiHint, ":") {
		uiHint = "http://127.0.0.1" + uiHint + "/"
	}
	slog.Info("server starting",
		"version", version,
		"commit", commit,
		"date", date,
		"addr", cfg.Addr,
		"db", cfg.DBPath,
		"upstream", cfg.UpstreamBase,
		"api_key_enabled", cfg.APIKey != "",
		"ui", uiHint,
	)
	if err := r.Run(cfg.Addr); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func mountWebUI(r *gin.Engine) {
	// Prefer on-disk ./web for easy local edits; else embedded copy.
	diskDir := resolveWebDir()
	if diskDir != "" {
		r.GET("/", func(c *gin.Context) {
			c.File(filepath.Join(diskDir, "index.html"))
		})
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
				return
			}
			// SPA-ish fallback
			c.File(filepath.Join(diskDir, "index.html"))
		})
		slog.Info("web ui from disk", "dir", diskDir)
		return
	}

	sub, err := fs.Sub(web.Content, ".")
	if err != nil {
		slog.Error("embed web failed", "err", err)
		return
	}
	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("index.html", http.FS(sub))
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
			return
		}
		c.FileFromFS("index.html", http.FS(sub))
	})
	slog.Info("web ui from embed")
}

func resolveWebDir() string {
	candidates := []string{"web", "./web"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "web"))
	}
	for _, d := range candidates {
		if st, err := os.Stat(filepath.Join(d, "index.html")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(d)
			return abs
		}
	}
	return ""
}

func requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client", c.ClientIP(),
		)
	}
}
