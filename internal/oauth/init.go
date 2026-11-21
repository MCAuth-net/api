package oauth

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type InitRequestQuery struct {
	ClientID    int64  `query:"client_id"`
	RedirectURI string `query:"redirect_uri"`
}

func Init(c fiber.Ctx, queries *database.Queries) error {
	query, err := helpers.ParseQueries[InitRequestQuery](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	u, err := url.Parse(query.RedirectURI)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	oauth, err := queries.GetOAuth(ctx, query.ClientID)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	if !slices.Contains(oauth.RedirectUris, query.RedirectURI) {
		return c.SendStatus(403)
	}

	q := u.Query()
	q.Set("client_id", strconv.FormatInt(query.ClientID, 10))
	q.Set("redirect_uri", query.RedirectURI)
	q.Set("name", oauth.Name.String)
	u.RawQuery = q.Encode()

	return c.Redirect().To(u.String())
}

type ConfirmRequest struct {
	ClientID    int64  `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
}

func Confirm(c fiber.Ctx, account helpers.AuthedAccount, queries *database.Queries, rdb *redis.Client) error {
	body, err := helpers.ParseBody[ConfirmRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	u, err := url.Parse(body.RedirectURI)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	oauth, err := queries.GetOAuth(ctx, body.ClientID)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	if !slices.Contains(oauth.RedirectUris, body.RedirectURI) {
		return c.SendStatus(403)
	}

	code, err := helpers.GenerateToken()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	err = rdb.Set(ctx, fmt.Sprintf("oauth:code:%s", code), account.Account.ID, time.Minute).Err()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	query := u.Query()
	query.Set("code", code)
	u.RawQuery = query.Encode()

	return c.Redirect().To(u.String())
}
