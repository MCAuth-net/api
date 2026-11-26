package tfa

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

func AuthWithEmailStart(c fiber.Ctx, account helpers.AuthedAccount, rdb *redis.Client) error {
	if !slices.Contains(account.Account.TfaMethods, "email") {
		return helpers.SendError(c, 405, "2FA method not allowed")
	}

	ip, err := netip.ParseAddr(c.IP())
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	code, err := helpers.GenerateOTP()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	err = rdb.Set(ctx, fmt.Sprintf("auth:email:%s", account.Token), code, 5*time.Minute).Err()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	err = apiutil.SendEmail([]string{account.Account.Email.String}, "One-time Auth Code", "tfa", map[string]any{
		"code": code,
		"ip":   ip.String(),
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}

type AuthWithEmailEndRequest struct {
	Code string `json:"code"`
}

func AuthWithEmailEnd(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	if !slices.Contains(account.Account.TfaMethods, "email") {
		return helpers.SendError(c, 405, "2FA method not allowed")
	}

	body, err := apiutil.ParseBody[AuthWithEmailEndRequest](c)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	code, err := rdb.Get(ctx, fmt.Sprintf("auth:email:%s", account.Token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return helpers.SendError(c, 401, "Invalid code")
		}
		return apiutil.HandleError(c, err)
	}

	if code == "" || code != body.Code {
		return helpers.SendError(c, 401, "Invalid code")
	}

	err = rdb.Del(ctx, fmt.Sprintf("auth:email:%s", account.Token)).Err()
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	err = helpers.EnterSudo(ctx, account, queries, rdb, "email")
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}
