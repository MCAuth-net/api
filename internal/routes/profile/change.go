package profile

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/helpers/email"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
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
		if err != nil || !match {
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

	var oldEmailPhrase string
	if account.Account.Email.Valid && account.Account.Email.String != "" {
		oldEmailPhrase = fmt.Sprintf(" from %s", account.Account.Email.String)
	}

	err = email.SendHTMLTemplate([]string{body.NewEmail}, "Change Email", "change_email_code", map[string]any{
		"code":             code,
		"old_email_phrase": oldEmailPhrase,
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

	_, err = queries.SetEmail(ctx, database.SetEmailParams{
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
