package helpers

import (
	"MCAuth-api/internal/database"
	"errors"
	"time"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const accountKey = "account"

type AuthedAccount struct {
	Account database.GetAccountFromSessionTokenRow
	UUIDs   []pgtype.UUID
}

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
			return apiutil.HandleError(c, err)
		}

		if time.Now().After(account.Expires.Time) {
			_, err = queries.RemoveSession(c.Context(), token)
			if err != nil {
				return apiutil.HandleError(c, err)
			}
			return c.SendStatus(401)
		}

		uuids, err := queries.GetUUIDsByAccountID(c.Context(), account.ID)
		if err != nil {
			return apiutil.HandleError(c, err)
		}

		if !account.Uuid.Valid && len(uuids) > 0 {
			_, err := queries.SetPrimaryUUID(c.Context(), database.SetPrimaryUUIDParams{
				Uuid:      uuids[0],
				AccountID: account.ID,
			})
			if err != nil {
				return apiutil.HandleError(c, err)
			}
			account.Uuid = uuids[0]
		}

		c.Locals(accountKey, AuthedAccount{Account: account, UUIDs: uuids})
		return c.Next()
	}
}

func Map(handler func(c fiber.Ctx, account AuthedAccount) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		authed, ok := c.Locals(accountKey).(AuthedAccount)
		if !ok {
			return c.SendStatus(401)
		}

		return handler(c, authed)
	}
}
