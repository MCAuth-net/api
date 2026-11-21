package oauth

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type GetTokenRequest struct {
	Code         string `json:"code"`
	ClientSecret string `json:"secret"`
}

func GetToken(c fiber.Ctx, queries *database.Queries, rdb *redis.Client) error {
	body, err := helpers.ParseBody[GetTokenRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	secret := helpers.SHA256(body.ClientSecret)

	ctx, cancel := helpers.GetContext()
	defer cancel()

	oauth, err := queries.GetOAuthFromSecret(ctx, secret)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.SendStatus(401)
		}
		return helpers.HandleError(c, err)
	}

	id, err := rdb.Get(ctx, fmt.Sprintf("oauth:code:%s", body.Code)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return c.SendStatus(401)
		}
		return helpers.HandleError(c, err)
	}

	token, err := helpers.GenerateToken()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	aid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	_, err = queries.CreateOAuthSession(ctx, database.CreateOAuthSessionParams{
		OauthID:   oauth.ID,
		AccountID: aid,
		Token:     token,
	})
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.Status(200).SendString(token)
}
