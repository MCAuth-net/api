package main

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	redisStorage "github.com/gofiber/storage/redis/v3"
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

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"https://mcauth.net",
			"http://localhost:3000",
		},
	}))

	store := redisStorage.NewFromConnection(rdb)

	v1 := app.Group("/v1", limiter.New(limiter.Config{
		Max:        60,
		Expiration: time.Minute,
		Storage:    store,
		Next: func(c fiber.Ctx) bool {
			return slices.ContainsFunc([]string{
				"/token",
				"/token/check",
			}, func(p string) bool {
				return c.Path() == "/v1"+p
			})
		},
		KeyGenerator: func(c fiber.Ctx) string {
			return fmt.Sprintf("v1:%s", c.IP())
		},
	}))

	type TokenResponse struct {
		Token    string `json:"token"`
		Mnemonic string `json:"mnemonic"`
	}
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
		mnemonic, err := helpers.GenerateMnemonic()
		if err != nil {
			return helpers.HandleError(c, err)
		}

		ctx, cancel := helpers.GetRedisContext()
		defer cancel()
		err = rdb.Set(ctx, fmt.Sprintf("mnemonic:%s", mnemonic), token, 5*time.Minute).Err()
		if err != nil {
			return helpers.HandleError(c, err)
		}

		return helpers.SendSuccess(c, &TokenResponse{
			Token:    token,
			Mnemonic: mnemonic,
		})
	})

	type TokenCheckRequest struct {
		Token string `json:"token"`
	}
	type TokenCheckResponse struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	v1.Post("/token/check", limiter.New(limiter.Config{
		Max:        3,
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
			if errors.Is(err, redis.Nil) {
				return c.SendStatus(404)
			}
			return helpers.HandleError(c, err)
		}

		verified, err := strconv.ParseBool(res)
		if err != nil {
			return helpers.HandleError(c, err)
		}
		if verified {
			uuid, err := queries.GetUUIDFromLoginToken(c.Context(), body.Token)
			if err != nil {
				return helpers.HandleError(c, err)
			}

			err = rdb.Del(ctx, fmt.Sprintf("token:%s", body.Token)).Err()
			if err != nil {
				return helpers.HandleError(c, err)
			}

			name, err := minecraft.GetName(uuid.String(), rdb)
			if err != nil {
				return helpers.HandleError(c, err)
			}

			return helpers.SendSuccess(c, &TokenCheckResponse{
				UUID: uuid.String(),
				Name: name,
			})
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
