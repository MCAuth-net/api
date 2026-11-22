package profile

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/helpers/email"
	"crypto/subtle"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email"`
	Password string `json:"password"`
}

func ChangeEmail(c fiber.Ctx, account helpers.AuthedAccount, rdb *redis.Client) error {
	body, err := helpers.ParseBody[ChangeEmailRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	if account.Account.Password.Valid {
		if body.Password == "" {
			return c.SendStatus(401)
		}
		match, err := helpers.Compare(body.Password, account.Account.Password.String)
		if err != nil {
			return helpers.HandleError(c, err)
		}
		if !match {
			return c.SendStatus(401)
		}
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	code, err := helpers.GenerateOTP()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	err = rdb.Set(ctx, fmt.Sprintf("new_email:%d", account.Account.ID), fmt.Sprintf("%s:%s", code, body.NewEmail), 5*time.Minute).Err()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	err = email.SendHTMLTemplate([]string{body.NewEmail}, "Change Email", "change_email_code", map[string]any{
		"code":      code,
		"old_email": account.Account.Email.String,
		"ip":        c.IP(),
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}

type ConfirmEmailRequest struct {
	Code string `json:"code"`
}

func ConfirmEmail(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	body, err := helpers.ParseBody[ConfirmEmailRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	key := fmt.Sprintf("new_email:%d", account.Account.ID)
	raw, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(body.Code)) != 1 {
		return c.SendStatus(401)
	}

	err = rdb.Del(ctx, key).Err()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	_, err = queries.ChangeEmail(ctx, database.ChangeEmailParams{
		ID: account.Account.ID,
		Email: pgtype.Text{
			String: parts[1],
			Valid:  true,
		},
	})

	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func ChangePassword(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := helpers.ParseBody[ChangePasswordRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	var lower, upper, digit, special bool

	for _, r := range body.NewPassword {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}

	length := utf8.RuneCountInString(body.NewPassword)
	if !lower || !upper || !digit || !special || length < 8 || length > 64 {
		return c.SendStatus(400)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	if account.Account.Password.Valid {
		old, err := helpers.Compare(body.OldPassword, account.Account.Password.String)
		if err != nil {
			return helpers.HandleError(c, err)
		}

		if !old {
			return c.SendStatus(401)
		}
	}

	hash, err := helpers.Hash(body.NewPassword)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	_, err = queries.ChangePassword(ctx, database.ChangePasswordParams{
		ID: account.Account.ID,
		Password: pgtype.Text{
			String: hash,
			Valid:  true,
		},
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}

type ChangeNameRequest struct {
	NewName string `json:"new_name"`
}

func ChangeName(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := helpers.ParseBody[ChangeNameRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	_, err = queries.ChangeName(ctx, database.ChangeNameParams{
		ID: account.Account.ID,
		Name: pgtype.Text{
			String: body.NewName,
			Valid:  true,
		},
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}

type SetPrimaryUUIDRequest struct {
	PrimaryUUID string `json:"primary_uuid"`
}

func SetPrimaryUUID(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := helpers.ParseBody[SetPrimaryUUIDRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	u, err := uuid.Parse(body.PrimaryUUID)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	_, err = queries.SetPrimaryUUID(ctx, database.SetPrimaryUUIDParams{
		Uuid: pgtype.UUID{
			Bytes: u,
			Valid: true,
		},
		AccountID: account.Account.ID,
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}

type ChangeColorRequest struct {
	NewColor string `json:"new_color"`
}

func ChangeColor(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := helpers.ParseBody[ChangeColorRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	if !slices.Contains([]string{
		"green",
		"blue",
		"teal",
		"violet",
		"pink",
		"yellow",
		"red",
		"gray",
	}, body.NewColor) {
		return c.SendStatus(400)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	_, err = queries.ChangeColor(ctx, database.ChangeColorParams{
		ID:    account.Account.ID,
		Color: body.NewColor,
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}
