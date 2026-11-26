package profile

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
)

func Set2FA(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	if !account.Sudo() {
		return helpers.NeedsSudo(c)
	}

	name, err := minecraft.GetName(account.Account.Uuid.String(), rdb)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MCAuth",
		AccountName: name,
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err = queries.Set2FA(ctx, database.Set2FAParams{
		AccountID: account.Account.ID,
		Token:     key.Secret(),
	})
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return c.SendString(key.URL())
}

func Remove2FA(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries) error {
	if !account.Sudo() {
		return helpers.NeedsSudo(c)
	}

	ctx, cancel := apiutil.GetContext()
	defer cancel()

	_, err := queries.Remove2FA(ctx, account.Account.ID)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	return helpers.SendNoError(c)
}
