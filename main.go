package main

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/oauth"
	"embed"
	"log"
	"log/slog"
	"os"
	"strings"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	redisStorage "github.com/gofiber/storage/redis/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed all:migration
var migrations embed.FS

//go:embed all:template
var templates embed.FS

func main() {
	var logLevel slog.LevelVar

	level := slog.LevelInfo

	switch strings.ToUpper(os.Getenv("LOG_LEVEL")) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}

	logLevel.Set(level)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: &logLevel,
	})))

	log.Println("Starting API")

	api, err := apiutil.Init(apiutil.Config{
		GooseFS:  migrations,
		GooseDir: "migration",
		FiberConfig: fiber.Config{
			ProxyHeader: apiutil.GetEnv("APP_PROXY_HEADER", fiber.HeaderXForwardedFor),
			TrustProxy:  apiutil.GetEnv("APP_TRUST_PROXY", "false") == "true",
			TrustProxyConfig: fiber.TrustProxyConfig{
				Proxies:  strings.Split(apiutil.GetEnv("APP_TRUSTED_PROXIES", "127.0.0.1/32,::/128"), ","),
				Loopback: true,
			},
		},
		EmailConfig: apiutil.EmailConfig{
			FS: templates,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	app := api.App
	pool := api.Pool
	queries := database.New(pool)
	rdb := api.RDB

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"https://mcauth.net",
			"http://localhost:3000",
		},
		AllowCredentials: true,
	}))

	store := redisStorage.NewFromConnection(rdb)

	v1 := app.Group("/v1")

	apiutil.BuildRoutes(v1, getRoutes(rdb, queries, store), helpers.Auth(queries, rdb))
	oauth.Init(v1.Group("/oauth2"), pool, rdb)

	if err := api.EasyListen(); err != nil {
		log.Fatal(err)
	}
}
