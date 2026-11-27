package profile

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

type ChangeEmailRequest struct {
	NewEmail string `json:"new_email"`
	Password string `json:"password"`
}

func ChangeEmail(c fiber.Ctx, account helpers.AuthedAccount, rdb *redis.Client) error {
	body, err := apiutil.ParseBody[ChangeEmailRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if account.Account.Password.Valid {
		if body.Password == "" {
			return helpers.WrongBody(c)
		}
		match, err := helpers.Compare(body.Password, account.Account.Password.String)
		if err != nil {
			return apiutil.HandleError(c, err)
		}
		if !match {
			return helpers.SendError(c, 401, "Wrong password")
		}
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	code, err := helpers.GenerateOTP()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	err = rdb.Set(ctx, fmt.Sprintf("new_email:%d", account.Account.ID), fmt.Sprintf("%s:%s", code, strings.ToLower(body.NewEmail)), 5*time.Minute).Err()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	err = apiutil.SendEmail([]string{body.NewEmail}, "Change Email", "change_email_code", map[string]any{
		"code":      code,
		"old_email": account.Account.Email.String,
		"ip":        c.IP(),
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type ConfirmEmailRequest struct {
	Code string `json:"code"`
}

func ConfirmEmail(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	body, err := apiutil.ParseBody[ConfirmEmailRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	key := fmt.Sprintf("new_email:%d", account.Account.ID)
	raw, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(body.Code)) != 1 {
		return helpers.SendError(c, 401, "Wrong code")
	}

	err = rdb.Del(ctx, key).Err()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	_, err = queries.ChangeEmail(ctx, database.ChangeEmailParams{
		ID: account.Account.ID,
		Email: pgtype.Text{
			String: parts[1],
			Valid:  true,
		},
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return helpers.SendError(c, 403, "Email already exists")
		}
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func ChangePassword(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[ChangePasswordRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
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
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	if account.Account.Password.Valid {
		old, err := helpers.Compare(body.OldPassword, account.Account.Password.String)
		if err != nil {
			return apiutil.HandleError(c, err)
		}

		if !old {
			return helpers.SendError(c, 401, "Wrong password")
		}
	}

	hash, err := helpers.Hash(body.NewPassword)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	_, err = queries.ChangePassword(ctx, database.ChangePasswordParams{
		ID: account.Account.ID,
		Password: pgtype.Text{
			String: hash,
			Valid:  true,
		},
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type ChangeNameRequest struct {
	NewName string `json:"new_name"`
}

func ChangeName(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[ChangeNameRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.ChangeName(ctx, database.ChangeNameParams{
		ID: account.Account.ID,
		Name: pgtype.Text{
			String: body.NewName,
			Valid:  true,
		},
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type SetPrimaryUUIDRequest struct {
	PrimaryUUID string `json:"primary_uuid"`
}

func SetPrimaryUUID(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[SetPrimaryUUIDRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	u, err := uuid.Parse(body.PrimaryUUID)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.SetPrimaryUUID(ctx, database.SetPrimaryUUIDParams{
		Uuid: pgtype.UUID{
			Bytes: u,
			Valid: true,
		},
		AccountID: account.Account.ID,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type ChangeColorRequest struct {
	NewColor string `json:"new_color"`
}

func ChangeColor(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	body, err := apiutil.ParseBody[ChangeColorRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
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
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.ChangeColor(ctx, database.ChangeColorParams{
		ID:    account.Account.ID,
		Color: body.NewColor,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

func ChangePfp(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	file, err := c.FormFile("pfp")
	if err != nil {
		return helpers.WrongBody(c)
	}

	if file.Size > 1048576 {
		return helpers.SendError(c, 413, "File too large")
	}

	open, err := file.Open()
	if err != nil {
		return apiutil.HandleError(c, err)
	}
	defer open.Close()

	bytes, err := io.ReadAll(open)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	mime := http.DetectContentType(bytes)
	if !strings.HasPrefix(mime, "image/") {
		return helpers.WrongBody(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.ChangePfp(ctx, database.ChangePfpParams{
		AccountID: account.Account.ID,
		Pfp:       bytes,
		Mime:      mime,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type Change2FARequest struct {
	Methods []string `json:"methods"`
}

func Change2FA(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	if !account.Sudo() {
		return helpers.NeedsSudo(c)
	}

	body, err := apiutil.ParseBody[Change2FARequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	for _, method := range body.Methods {
		if !slices.Contains([]string{
			"app",
			"email",
		}, method) {
			return helpers.WrongBody(c)
		}
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.Change2FAMethods(ctx, database.Change2FAMethodsParams{
		ID:         account.Account.ID,
		TfaMethods: body.Methods,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}
