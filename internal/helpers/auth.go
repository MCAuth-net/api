package helpers

import (
	"MCAuth-api/internal/database"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
)

const accountKey = "account"

func Auth(queries *database.Queries) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Get(fiber.HeaderAuthorization)
		if token == "" {
			token = c.Cookies("session")
		}
		if token == "" {
			return c.SendStatus(401)
		}

		account, err := queries.GetAccountFromSessionToken(c.Context(), token)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.SendStatus(401)
			}
			return HandleError(c, err)
		}

		c.Locals(accountKey, account)
		return c.Next()
	}
}

func Map(handler func(c fiber.Ctx, account database.Account) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		account, ok := c.Locals(accountKey).(database.Account)
		if !ok {
			return c.SendStatus(401)
		}

		return handler(c, account)
	}
}
