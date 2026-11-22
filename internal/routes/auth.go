package routes

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"MCAuth-api/internal/helpers/email"
	"MCAuth-api/internal/minecraft"
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func CreateSession(c fiber.Ctx, ctx context.Context, queries *database.Queries, id int64, account *database.Account) error {
	ip, err := netip.ParseAddr(c.IP())
	if err != nil {
		return err
	}

	token, err := helpers.GenerateToken()
	if err != nil {
		return err
	}

	if account == nil {
		res, err := queries.GetAccount(ctx, id)
		if err != nil {
			return err
		}
		account = &res
	}

	agent := c.UserAgent()
	if account.Email.Valid {
		parsed := helpers.ParseUserAgent(agent)

		err = email.SendHTMLTemplate([]string{account.Email.String}, "New login", "new_login", map[string]any{
			"browser": fmt.Sprintf("%s %s", parsed.Browser, parsed.BrowserVersion),
			"os":      fmt.Sprintf("%s %s", parsed.OS, parsed.OSVersion),
			"device":  parsed.Device,
			"ip":      ip.String(),
		})
		if err != nil {
			return err
		}
	}

	session, err := queries.CreateSession(ctx, database.CreateSessionParams{
		AccountID: id,
		Token:     token,
		Ip:        ip,
		Agent:     agent,
	})
	if err != nil {
		return err
	}

	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    token,
		Domain:   ".mcauth.net",
		Path:     "/",
		Expires:  session.Time,
		Secure:   true,
		HTTPOnly: true,
		SameSite: "Lax",
	})
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func Login(c fiber.Ctx, queries *database.Queries) error {
	body, err := helpers.ParseBody[LoginRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	var account database.Account
	switch {
	case body.Email != "":
		account, err = queries.GetAccountByEmail(ctx, pgtype.Text{
			String: strings.ToLower(body.Email),
			Valid:  true,
		})
		if err != nil {
			return c.SendStatus(401)
		}
	case body.Name != "":
		id, err := minecraft.GetUUID(body.Name)
		if err != nil {
			return helpers.HandleError(c, err)
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			return helpers.HandleError(c, err)
		}

		account, err = queries.GetAccountByUUID(ctx, pgtype.UUID{
			Bytes: parsed,
			Valid: true,
		})
		if err != nil {
			return helpers.HandleError(c, err)
		}
	default:
		return c.SendStatus(400)
	}

	match, err := helpers.Compare(body.Password, account.Password.String)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	if !match {
		return c.SendStatus(401)
	}

	err = CreateSession(c, ctx, queries, account.ID, &account)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}
