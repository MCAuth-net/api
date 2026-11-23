package main

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/routes"
	"MCAuth-api/internal/routes/client"
	"MCAuth-api/internal/routes/profile"
	"fmt"
	"time"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/redis/go-redis/v9"
)

func getRoutes(rdb *redis.Client, queries *database.Queries, store fiber.Storage) fiber.Map {
	return fiber.Map{
		apiutil.Limiter: limiter.New(limiter.Config{
			Max:        60,
			Expiration: time.Minute,
			Storage:    store,
			KeyGenerator: func(c fiber.Ctx) string {
				return fmt.Sprintf("v1:%s", c.IP())
			},
		}),
		apiutil.NeedsAuth: false,

		"token": fiber.Map{
			apiutil.Limiter: limiter.New(limiter.Config{
				Max:        5,
				Expiration: 5 * time.Minute,
				Storage:    store,
				KeyGenerator: func(c fiber.Ctx) string {
					return fmt.Sprintf("v1:token:%s", c.IP())
				},
			}),
			fiber.MethodGet: func(c fiber.Ctx) error { return routes.Token(c, rdb) },

			"check": fiber.Map{
				apiutil.Limiter: limiter.New(limiter.Config{
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
		"profile": fiber.Map{
			apiutil.NeedsAuth: true,
			"me":              apiutil.Get(helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error { return profile.GetMe(c, account, queries, rdb) })),
			"pfp":             apiutil.Get(helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error { return profile.GetPfp(c, account, queries) })),
			"change": fiber.Map{
				apiutil.Limiter: limiter.New(limiter.Config{
					Max:        3,
					Expiration: time.Hour,
					Storage:    store,
					KeyGenerator: func(c fiber.Ctx) string {
						return fmt.Sprintf("v1:profile:change:%s:%s", c.Path(), c.IP())
					},
				}),
				"email": fiber.Map{
					fiber.MethodPost: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error { return profile.ChangeEmail(c, account, rdb) }),
					"confirm": fiber.Map{
						apiutil.Limiter: limiter.New(limiter.Config{
							Max:        10,
							Expiration: time.Minute,
							Storage:    store,
							KeyGenerator: func(c fiber.Ctx) string {
								return fmt.Sprintf("v1:profile:change:confirm:%s", c.IP())
							},
						}),
						fiber.MethodPost: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
							return profile.ConfirmEmail(c, account, queries, rdb)
						}),
					},
				},
				"password": apiutil.Post(helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
					return profile.ChangePassword(c, account, queries)
				})),
				"name": fiber.Map{
					apiutil.Limiter: limiter.New(limiter.Config{
						Max:        5,
						Expiration: time.Minute,
						Storage:    store,
						KeyGenerator: func(c fiber.Ctx) string {
							return fmt.Sprintf("v1:profile:change:%s:%s", c.Path(), c.IP())
						},
					}),
					fiber.MethodPost: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error { return profile.ChangeName(c, account, queries) }),
				},
				"uuid": fiber.Map{
					apiutil.Limiter: limiter.New(limiter.Config{
						Max:        5,
						Expiration: time.Minute,
						Storage:    store,
						KeyGenerator: func(c fiber.Ctx) string {
							return fmt.Sprintf("v1:profile:change:%s:%s", c.Path(), c.IP())
						},
					}),
					fiber.MethodPost: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
						return profile.SetPrimaryUUID(c, account, queries)
					}),
				},
				"color": fiber.Map{
					apiutil.Limiter: limiter.New(limiter.Config{
						Max:        5,
						Expiration: time.Minute,
						Storage:    store,
						KeyGenerator: func(c fiber.Ctx) string {
							return fmt.Sprintf("v1:profile:change:%s:%s", c.Path(), c.IP())
						},
					}),
					fiber.MethodPost: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
						return profile.ChangeColor(c, account, queries)
					}),
				},
				"pfp": apiutil.Post(helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error { return profile.ChangePfp(c, account, queries) })),
			},
		},
		"client": fiber.Map{
			apiutil.Limiter: limiter.New(limiter.Config{
				Max:        5,
				Expiration: time.Minute,
				Storage:    store,
				KeyGenerator: func(c fiber.Ctx) string {
					return fmt.Sprintf("v1:client:client:%s:%s", c.Method(), c.IP())
				},
			}),
			apiutil.NeedsAuth: true,
			fiber.MethodGet: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
				return client.ReadClient(c, account, queries)
			}),
			fiber.MethodPost: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
				return client.CreateClient(c, account, queries)
			}),
			fiber.MethodPatch: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
				return client.UpdateClient(c, account, queries)
			}),
			fiber.MethodDelete: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
				return client.DeleteClient(c, account, queries)
			}),
			fiber.MethodPut: helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error {
				return client.UpdateSecret(c, account, queries)
			}),
		},
		"clients": fiber.Map{
			apiutil.NeedsAuth: true,
			fiber.MethodGet:   helpers.Map(func(c fiber.Ctx, account helpers.AuthedAccount) error { return client.ListClients(c, account, queries) }),
		},
		"auth": fiber.Map{
			apiutil.Limiter: limiter.New(limiter.Config{
				Max:        5,
				Expiration: 5 * time.Minute,
				Storage:    store,
				KeyGenerator: func(c fiber.Ctx) string {
					return fmt.Sprintf("v1:auth:%s", c.IP())
				},
			}),
			fiber.MethodPost: func(c fiber.Ctx) error { return routes.Login(c, queries) },
			fiber.MethodDelete: func(c fiber.Ctx) error {
				ctx, cancel := apiutil.GetContext()
				defer cancel()

				_, err := queries.RemoveSession(ctx, c.Cookies("session"))
				if err != nil {
					return apiutil.HandleError(c, err)
				}
				c.ClearCookie("session")

				return c.SendStatus(200)
			},
		},
	}
}
