package routes

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/minecraft"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type TokenResponse struct {
	Token    string `json:"token"`
	Mnemonic string `json:"mnemonic"`
}

func Token(c fiber.Ctx, rdb *redis.Client) error {
	token, err := helpers.GenerateToken()
	if err != nil {
		return helpers.HandleError(c, err)
	}
	mnemonic, err := helpers.GenerateMnemonic()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()
	err = rdb.Set(ctx, fmt.Sprintf("mnemonic:%s", mnemonic), token, 5*time.Minute).Err()
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.SendSuccess(c, &TokenResponse{
		Token:    token,
		Mnemonic: mnemonic,
	})
}

type TokenCheckRequest struct {
	Token string `json:"token"`
}
type TokenCheckResponse struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func TokenCheck(c fiber.Ctx, queries *database.Queries, rdb *redis.Client) error {
	body, err := helpers.ParseBody[TokenCheckRequest](c)
	if err != nil {
		return c.SendStatus(400)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()
	res, err := rdb.Get(ctx, fmt.Sprintf("token:%s", body.Token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return c.SendStatus(404)
		}
		return helpers.HandleError(c, err)
	}

	verified, err := strconv.ParseBool(res)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	if verified {
		uuid, err := queries.GetUUIDFromLoginToken(c.Context(), body.Token)
		if err != nil {
			return helpers.HandleError(c, err)
		}

		err = rdb.Del(ctx, fmt.Sprintf("token:%s", body.Token)).Err()
		if err != nil {
			return helpers.HandleError(c, err)
		}

		name, err := minecraft.GetName(uuid.String(), rdb)
		if err != nil {
			return helpers.HandleError(c, err)
		}

		account, err := queries.GetAccountByUUID(ctx, uuid)
		if err != nil {
			return helpers.HandleError(c, err)
		}

		err = CreateSession(c, ctx, queries, account, nil)
		if err != nil {
			return helpers.HandleError(c, err)
		}
		return helpers.SendSuccess(c, &TokenCheckResponse{
			UUID: uuid.String(),
			Name: name,
		})
	}

	return c.SendStatus(404)
}
