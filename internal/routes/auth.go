package routes

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"
	"errors"
	"strings"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func Login(c fiber.Ctx, queries *database.Queries) error {
	body, err := apiutil.ParseBody[LoginRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	var account database.Account
	switch {
	case body.Email != "":
		account, err = queries.GetAccountByEmail(ctx, pgtype.Text{
			String: strings.ToLower(body.Email),
			Valid:  true,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return helpers.SendError(c, 404, "Email not found")
			}
			return helpers.SendError(c, 401, "Login failed")
		}
	case body.Name != "":
		id, err := minecraft.GetUUID(body.Name)
		if err != nil {
			return apiutil.HandleError(c, err)
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			return apiutil.HandleError(c, err)
		}

		account, err = queries.GetAccountByUUID(ctx, pgtype.UUID{
			Bytes: parsed,
			Valid: true,
		})
		if err != nil {
			return apiutil.HandleError(c, err)
		}
	default:
		return helpers.WrongBody(c)
	}

	match, err := helpers.Compare(body.Password, account.Password.String)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if !match {
		return helpers.SendError(c, 401, "Login failed")
	}

	err = helpers.CreateSession(c, ctx, queries, account.ID, &account)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}
