package profile

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func GetMe(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	uuids := make(map[string]string, len(account.UUIDs))

	for _, u := range account.UUIDs {
		if !u.Valid {
			continue
		}

		id := uuid.UUID(u.Bytes).String()

		name, err := minecraft.GetName(id, rdb)
		if err != nil {
			return apiutil.HandleError(c, err)
		}

		uuids[id] = name
	}

	name, err := minecraft.GetName(account.Account.Uuid.String(), rdb)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	pfp, err := queries.HasPfp(ctx, account.Account.ID)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":           account.Account.ID,
		"email":        account.Account.Email,
		"uuid":         account.Account.Uuid,
		"name":         name,
		"display_name": account.Account.Name,
		"color":        account.Account.Color,
		"uuids":        uuids,
		"pfp":          pfp,
	})
}

func GetPfp(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	ctx, cancel := apiutil.GetContext()
	defer cancel()

	pfp, err := queries.GetPfp(ctx, account.Account.ID)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	c.Set("Content-Type", pfp.Mime)
	return c.Send(pfp.Pfp)
}
