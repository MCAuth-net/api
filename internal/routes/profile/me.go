package profile

import (
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func GetMe(c fiber.Ctx, account helpers.AuthedAccount, rdb *redis.Client) error {
	uuids := make(map[string]string, len(account.UUIDs))

	for _, u := range account.UUIDs {
		if !u.Valid {
			continue
		}

		id := uuid.UUID(u.Bytes).String()

		name, err := minecraft.GetName(id, rdb)
		if err != nil {
			return helpers.HandleError(c, err)
		}

		uuids[id] = name
	}

	name, err := minecraft.GetName(account.Account.Uuid.String(), rdb)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":           account.Account.ID,
		"email":        account.Account.Email,
		"uuid":         account.Account.Uuid,
		"name":         name,
		"display_name": account.Account.Name,
		"color":        account.Account.Color,
		"uuids":        uuids,
	})
}
