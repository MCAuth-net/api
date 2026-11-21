package oauth

import (
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func unauthorized(c fiber.Ctx) error {
	c.Set(fiber.HeaderWWWAuthenticate, `Bearer error="invalid_token"`)
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_token"})
}

func userinfo(rdb *redis.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, "/", nil)
		if err != nil {
			return helpers.HandleError(c, err)
		}
		req.Header.Set("Authorization", c.Get(fiber.HeaderAuthorization))

		ti, err := srv.ValidationBearerToken(req)
		if err != nil {
			return unauthorized(c)
		}

		accountID, err := strconv.ParseInt(ti.GetUserID(), 10, 64)
		if err != nil {
			return unauthorized(c)
		}

		account, err := db.GetAccountWithUUID(c.Context(), accountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return unauthorized(c)
			}
			return helpers.HandleError(c, err)
		}

		info := fiber.Map{
			"id":           account.ID,
			"display_name": account.Name,
		}

		if account.Uuid.Valid {
			id := uuid.UUID(account.Uuid.Bytes).String()
			name, err := minecraft.GetName(id, rdb)
			if err != nil {
				return helpers.HandleError(c, err)
			}
			info["uuid"] = id
			info["name"] = name
		}

		if slices.Contains(strings.Fields(ti.GetScope()), "email") {
			info["email"] = account.Email
		}

		return c.JSON(info)
	}
}
