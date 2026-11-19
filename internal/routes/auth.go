package routes

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"context"
	"net/netip"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgtype"
)

func CreateSession(c fiber.Ctx, ctx context.Context, queries *database.Queries, id int64) error {
	ip, err := netip.ParseAddr(c.IP())
	if err != nil {
		return err
	}

	token, err := helpers.GenerateToken()
	if err != nil {
		return err
	}

	session, err := queries.CreateSession(ctx, database.CreateSessionParams{
		AccountID: id,
		Token:     token,
		Ip:        ip,
		Agent:     c.UserAgent(),
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
	Password string `json:"password"`
}

func Login(c fiber.Ctx, queries *database.Queries) error {
	body, err := helpers.ParseBody[LoginRequest](c)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	ctx, cancel := helpers.GetContext()
	defer cancel()

	account, err := queries.GetAccountByEmail(ctx, pgtype.Text{
		String: body.Email,
		Valid:  true,
	})
	if err != nil {
		return c.SendStatus(401)
	}

	match, err := helpers.Compare(body.Password, account.Password.String)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	if !match {
		return c.SendStatus(401)
	}

	err = CreateSession(c, ctx, queries, account.ID)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.SendStatus(200)
}
