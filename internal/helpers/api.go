package helpers

import (
	"context"
	"log/slog"
	"time"

	"github.com/goccy/go-json/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/mssola/useragent"
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

func ParseQueries[T any](c fiber.Ctx) (T, error) {
	var queries T

	err := c.Bind().Query(&queries)
	if err != nil {
		return queries, err
	}

	return queries, nil
}

func GetContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func SendSuccess(c fiber.Ctx, data any) error {
	dat, err := json.Marshal(data)
	if err != nil {
		return HandleError(c, err)
	}
	c.Set("Content-Type", "application/json")
	return c.Status(200).Send(dat)
}

type UserAgentInfo struct {
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	OS             string `json:"os"`
	OSVersion      string `json:"os_version"`
	Platform       string `json:"platform"`
	Device         string `json:"device"`
	Mobile         bool   `json:"mobile"`
	Bot            bool   `json:"bot"`
}

func ParseUserAgent(value string) UserAgentInfo {
	ua := useragent.New(value)

	browser, browserVersion := ua.Browser()
	os := ua.OSInfo()

	return UserAgentInfo{
		Browser:        browser,
		BrowserVersion: browserVersion,
		OS:             os.Name,
		OSVersion:      os.Version,
		Platform:       ua.Platform(),
		Device:         ua.Model(),
		Mobile:         ua.Mobile(),
		Bot:            ua.Bot(),
	}
}
