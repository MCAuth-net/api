package tfa

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"slices"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
)

type AuthWithAppRequest struct {
	Code string `json:"code"`
}

func AuthWithApp(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	if !slices.Contains(account.Account.TfaMethods, "app") {
		return helpers.SendError(c, 405, "2FA method not allowed")
	}

	body, err := apiutil.ParseBody[AuthWithAppRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	token, err := queries.Get2FA(ctx, account.Account.ID)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	if !totp.Validate(body.Code, token) {
		return helpers.SendError(c, 401, "Wrong 2FA code")
	}

	err = helpers.EnterSudo(ctx, account, queries, rdb, "app")
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}
