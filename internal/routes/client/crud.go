package client

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"errors"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateClientRequest struct {
	Name string `json:"name"`
}

func CreateClient(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[CreateClientRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if body.Name == "" {
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	token, err := helpers.GenerateToken()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	id, err := queries.CreateOAuth(ctx, database.CreateOAuthParams{
		Name: pgtype.Text{
			String: body.Name,
			Valid:  true,
		},
		Secret: helpers.SHA256(token),
		Owner:  account.Account.ID,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":     id,
		"secret": token,
	})
}

type ReadClientRequest struct {
	ID int64 `json:"id"`
}

func ReadClient(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[ReadClientRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if body.ID == 0 {
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	client, err := queries.GetOAuthWithOwner(ctx, database.GetOAuthWithOwnerParams{
		ID:    body.ID,
		Owner: account.Account.ID,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":            client.ID,
		"name":          client.Name,
		"redirect_uris": client.RedirectUris,
		"owner":         client.Owner,
	})
}

type UpdateClientRequest struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	RedirectURI []string `json:"redirect_uris"`
}

func UpdateClient(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[UpdateClientRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if body.ID == 0 {
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	client, err := queries.GetOAuthWithOwner(ctx, database.GetOAuthWithOwnerParams{
		ID:    body.ID,
		Owner: account.Account.ID,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	name := body.Name
	if name == "" {
		name = client.Name.String
	}
	redirectURIs := body.RedirectURI
	if redirectURIs == nil {
		redirectURIs = client.RedirectUris
	}

	_, err = queries.UpdateOAuth(ctx, database.UpdateOAuthParams{
		ID: client.ID,
		Name: pgtype.Text{
			String: name,
			Valid:  true,
		},
		RedirectUris: redirectURIs,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type DeleteClientRequest struct {
	ID int64 `json:"id"`
}

func DeleteClient(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[DeleteClientRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if body.ID == 0 {
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.DeleteOAuthWithOwner(ctx, database.DeleteOAuthWithOwnerParams{
		ID:    body.ID,
		Owner: account.Account.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return helpers.WrongBody(c)
		}
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}
