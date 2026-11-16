package helpers

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

func HandleError(c fiber.Ctx, err error) error {
	slog.Debug(err.Error())
	return c.SendStatus(500)
}

func ParseBody[T any](c fiber.Ctx) (T, error) {
	var body T

	err := c.Bind().Body(&body)
	if err != nil {
		return body, err
	}

	return body, nil
}

func GetRedisContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
