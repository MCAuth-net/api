package main

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	redisStorage "github.com/gofiber/storage/redis/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

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

	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "8080")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		helpers.Getenv("POSTGRES_HOST", "localhost"),
		helpers.Getenv("POSTGRES_PORT", "5432"),
		os.Getenv("POSTGRES_DATABASE"),
	)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL: ", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("PostgreSQL ping failed: ", err)
	}

	log.Println("Connected to PostgreSQL")

	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal("Failed to open migration database: ", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		log.Fatal("Migration database ping failed: ", err)
	}

	if err := Migrate(sqlDB); err != nil {
		log.Fatal("Failed to run migrations: ", err)
	}

	log.Println("Database migrations completed")

	queries := database.New(pool)

	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		redisDB = 0
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     helpers.Getenv("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis ping failed: ", err)
	}

	log.Println("Connected to Redis")
	log.Println("Starting API")

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"https://mcauth.net",
			"http://localhost:3000",
		},
		AllowCredentials: true,
	}))

	store := redisStorage.NewFromConnection(rdb)

	helpers.BuildRoutes(app.Group("/v1"), getRoutes(rdb, queries, store), helpers.Auth(queries))

	if err := app.Listen(":" + os.Getenv("PORT")); err != nil {
		log.Fatal(err)
	}
}
