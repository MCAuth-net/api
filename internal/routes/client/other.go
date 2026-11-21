package client

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"

	"github.com/gofiber/fiber/v3"
)

func ListClients(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	ctx, cancel := helpers.GetContext()
	defer cancel()

	clients, err := queries.GetOAuths(ctx, account.Account.ID)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	result := make([]fiber.Map, 0, len(clients))

	for _, client := range clients {
		result = append(result, fiber.Map{
			"id":            client.ID,
			"name":          client.Name,
			"redirect_uris": client.RedirectUris,
			"owner":         client.Owner,
		})
	}

	return c.JSON(result)
}

type UpdateSecretRequest struct {
	ID int64 `json:"id"`
}

func UpdateSecret(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := helpers.ParseBody[UpdateSecretRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	if body.ID == 0 {
		return c.SendStatus(400)
	}

	token, err := helpers.GenerateToken()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	_, err = queries.UpdateOAuthSecretWithOwner(ctx, database.UpdateOAuthSecretWithOwnerParams{
		ID:     body.ID,
		Owner:  account.Account.ID,
		Secret: helpers.SHA256(token),
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.Status(200).SendString(token)
}
