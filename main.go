package main

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	redisStorage "github.com/gofiber/storage/redis/v3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func main() {
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "8080")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_PORT", "5432"),
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
	_ = queries

	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		redisDB = 0
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
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
	store := redisStorage.NewFromConnection(rdb)

	v1 := app.Group("/v1", limiter.New(limiter.Config{
		Max:        60,
		Expiration: time.Minute,
		Storage:    store,
		KeyGenerator: func(c fiber.Ctx) string {
			return fmt.Sprintf("v1:%s", c.IP())
		},
	}))

	v1.Get("/token", limiter.New(limiter.Config{
		Max:        5,
		Expiration: 5 * time.Minute,
		Storage:    store,
		KeyGenerator: func(c fiber.Ctx) string {
			return fmt.Sprintf("v1:token:%s", c.IP())
		},
	}), func(c fiber.Ctx) error {
		token, err := helpers.GenerateToken()
		if err != nil {
			return helpers.HandleError(c, err)
		}

		ctx, cancel := helpers.GetRedisContext()
		defer cancel()
		err = rdb.Set(ctx, fmt.Sprintf("token:%s", token), "false", 5*time.Minute).Err()
		if err != nil {
			return helpers.HandleError(c, err)
		}

		return c.Status(200).SendString(token)
	})

	type TokenCheckRequest struct {
		Token string `json:"token"`
	}
	v1.Post("/token/check", limiter.New(limiter.Config{
		Max:        2,
		Expiration: time.Second,
		Storage:    store,
		KeyGenerator: func(c fiber.Ctx) string {
			return fmt.Sprintf("v1:token:check:%s", c.IP())
		},
	}), func(c fiber.Ctx) error {
		body, err := helpers.ParseBody[TokenCheckRequest](c)
		if err != nil {
			return c.SendStatus(400)
		}

		ctx, cancel := helpers.GetRedisContext()
		defer cancel()
		res, err := rdb.Get(ctx, fmt.Sprintf("token:%s", body.Token)).Result()
		if err != nil {
			return helpers.HandleError(c, err)
		}

		verified, err := strconv.ParseBool(res)
		if err != nil {
			return helpers.HandleError(c, err)
		}
		if verified {
			err := rdb.Del(ctx, fmt.Sprintf("token:%s", body.Token)).Err()
			if err != nil {
				return helpers.HandleError(c, err)
			}
			uuid, err := queries.GetUUIDFromLoginToken(c.Context(), pgtype.Text{
				String: body.Token,
				Valid:  true,
			})
			if err != nil {
				return helpers.HandleError(c, err)
			}

			return c.Status(200).SendString(uuid.String())
		}

		return c.SendStatus(404)
	})

	if err := app.Listen(":" + os.Getenv("PORT")); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
