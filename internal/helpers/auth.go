package helpers

import (
	"MCAuth-api/internal/database"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"git.jgj52.dev/JGJ52/apiutil"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

const AccountKey = "account"

type AuthedAccount struct {
	Account database.GetAccountFromSessionTokenRow
	UUIDs   []pgtype.UUID
	Sudo    func() bool
	Token   string
}

func Auth(queries *database.Queries, rdb *redis.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Get(fiber.HeaderAuthorization)
		if token == "" {
			token = c.Cookies("session")
		}
		if token == "" {
			return SendError(c, 401, "No session token")
		}

		account, err := queries.GetAccountFromSessionToken(c.Context(), token)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SendError(c, 404, "Account not found")
			}
			return apiutil.HandleError(c, err)
		}

		if time.Now().After(account.Expires.Time) {
			_, err = queries.RemoveSession(c.Context(), token)
			if err != nil {
				return apiutil.HandleError(c, err)
			}
			return SendError(c, 403, "Session token expired")
		}

		uuids, err := queries.GetUUIDsByAccountID(c.Context(), account.ID)
		if err != nil {
			return apiutil.HandleError(c, err)
		}

		if !account.Uuid.Valid && len(uuids) > 0 {
			_, err := queries.SetPrimaryUUID(c.Context(), database.SetPrimaryUUIDParams{
				Uuid:      uuids[0],
				AccountID: account.ID,
			})
			if err != nil {
				return apiutil.HandleError(c, err)
			}
			account.Uuid = uuids[0]
		}

		c.Locals(AccountKey, AuthedAccount{
			Account: account,
			UUIDs:   uuids,
			Sudo: func() bool {
				if len(account.TfaMethods) == 0 {
					return true
				}
				sudo, err := rdb.Exists(c.Context(), fmt.Sprintf("sudo:%d", account.SessionID)).Result()
				if err != nil {
					slog.Debug(err.Error())
					return false
				}
				return sudo == 1
			},
			Token: token,
		})
		return c.Next()
	}
}

func Map(handler func(c fiber.Ctx, account AuthedAccount) error, doesntNeedActivated ...bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		authed, ok := c.Locals(AccountKey).(AuthedAccount)
		if !ok {
			return c.SendStatus(401)
		}

		if !authed.Account.Activated && !(len(doesntNeedActivated) > 0 && doesntNeedActivated[0]) {
			return SendError(c, 401, "Account not activated")
		}

		return handler(c, authed)
	}
}

func CreateSession(c fiber.Ctx, ctx context.Context, queries *database.Queries, id int64, account *database.Account) error {
	ip, err := netip.ParseAddr(c.IP())
	if err != nil {
		return err
	}

	token, err := GenerateToken()
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
		parsed := ParseUserAgent(agent)

		err = apiutil.SendEmail([]string{account.Email.String}, "New login", "new_login", map[string]any{
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
		Activated: len(account.TfaMethods) == 0,
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

func EnterSudo(ctx context.Context, account AuthedAccount, queries *database.Queries, rdb *redis.Client, method string) error {
	if !account.Account.Activated {
		_, err := queries.ActivateSession(ctx, database.ActivateSessionParams{
			Token:     account.Token,
			Activated: true,
		})
		if err != nil {
			return err
		}
	}
	err := rdb.Set(ctx, fmt.Sprintf("sudo:%d", account.Account.SessionID), method, 15*time.Minute).Err()
	if err != nil {
		return err
	}

	return nil
}
