package main

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/routes"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/redis/go-redis/v9"
)

func getRoutes(rdb *redis.Client, queries *database.Queries, store fiber.Storage) fiber.Map {
	return fiber.Map{
		helpers.Limiter: limiter.New(limiter.Config{
			Max:        60,
			Expiration: time.Minute,
			Storage:    store,
			KeyGenerator: func(c fiber.Ctx) string {
				return fmt.Sprintf("v1:%s", c.IP())
			},
		}),
		helpers.NeedsAuth: false,

		"token": fiber.Map{
			helpers.Limiter: limiter.New(limiter.Config{
				Max:        5,
				Expiration: 5 * time.Minute,
				Storage:    store,
				KeyGenerator: func(c fiber.Ctx) string {
					return fmt.Sprintf("v1:token:%s", c.IP())
				},
			}),
			fiber.MethodGet: func(c fiber.Ctx) error { return routes.Token(c, rdb) },

			"check": fiber.Map{
				helpers.Limiter: limiter.New(limiter.Config{
					Max:        3,
					Expiration: time.Second,
					Storage:    store,
					KeyGenerator: func(c fiber.Ctx) string {
						return fmt.Sprintf("v1:token:check:%s", c.IP())
					},
				}),
				fiber.MethodPost: func(c fiber.Ctx) error { return routes.TokenCheck(c, queries, rdb) },
			},
		},
	}
}
