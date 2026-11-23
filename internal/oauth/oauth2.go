package oauth

import (
	"MCAuth-api/internal/database"
	"MCAuth-api/internal/helpers"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"git.jgj52.dev/JGJ52/apiutil"
	oa "github.com/go-oauth2/oauth2/v4"
	oaerrors "github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const tokenLifetime = 30 * 24 * time.Hour

var (
	srv *server.Server
	db  *database.Queries
)

func Init(r fiber.Router, pool *pgxpool.Pool, rdb *redis.Client) {
	db = database.New(pool)

	manager := manage.NewDefaultManager()
	manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    tokenLifetime,
		IsGenerateRefresh: false,
	})
	manager.MapTokenStorage(&tokenStore{pool: pool, queries: db, rdb: rdb})
	manager.MapAccessGenerate(accessGenerate{})
	manager.MapClientStorage(&clientStore{queries: db})
	manager.SetValidateURIHandler(validateRedirectURI)

	cfg := server.NewConfig()
	cfg.AllowedResponseTypes = []oa.ResponseType{oa.Code}
	cfg.AllowedGrantTypes = []oa.GrantType{oa.AuthorizationCode}
	cfg.AllowedCodeChallengeMethods = []oa.CodeChallengeMethod{oa.CodeChallengeS256}
	cfg.ForcePKCE = true

	srv = server.NewServer(cfg, manager)

	r.Get("/authorize", helpers.Auth(db), helpers.Map(authorize))

	r.Post("/token", adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = srv.HandleTokenRequest(w, req)
	}))

	r.Get("/userinfo", userinfo(rdb))
}

func authorize(c fiber.Ctx, account helpers.AuthedAccount) error {
	r, err := adaptor.ConvertRequest(c, false)
	if err != nil {
		return apiutil.HandleError(c, err)
	}

	req, err := srv.ValidationAuthorizeRequest(r)
	if err != nil {
		return authorizeError(c, err)
	}
	req.UserID = strconv.FormatInt(account.Account.ID, 10)

	if _, err := resolveScopes(c.Context(), db, req.Scope); err != nil {
		return authorizeError(c, err)
	}

	ti, err := srv.GetAuthorizeToken(c.Context(), req)
	if err != nil {
		return authorizeError(c, err)
	}

	uri, err := srv.GetRedirectURI(req, srv.GetAuthorizeData(req.ResponseType, ti))
	if err != nil {
		return authorizeError(c, err)
	}

	return c.Redirect().To(uri)
}

func authorizeError(c fiber.Ctx, err error) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
}

func validateRedirectURI(baseURI, redirectURI string) error {
	if !slices.Contains(strings.Fields(baseURI), redirectURI) {
		return oaerrors.ErrInvalidRedirectURI
	}
	return nil
}

type accessGenerate struct{}

func (accessGenerate) Token(_ context.Context, _ *oa.GenerateBasic, _ bool) (string, string, error) {
	token, err := helpers.GenerateToken()
	return token, "", err
}

func resolveScopes(ctx context.Context, q *database.Queries, scope string) ([]string, error) {
	names := slices.Compact(slices.Sorted(slices.Values(strings.Fields(scope))))
	if len(names) == 0 {
		return nil, nil
	}

	found, err := q.GetScopes(ctx, names)
	if err != nil {
		slog.Error("oauth scope lookup failed", "err", err)
		return nil, oaerrors.ErrServerError
	}
	if len(found) != len(names) {
		return nil, oaerrors.ErrInvalidScope
	}

	return names, nil
}

type tokenStore struct {
	pool    *pgxpool.Pool
	queries *database.Queries
	rdb     *redis.Client
}

func codeKey(code string) string { return fmt.Sprintf("oauth:code:%s", code) }

func (s *tokenStore) Create(ctx context.Context, info oa.TokenInfo) error {
	if info.GetCode() != "" {
		return s.createCode(ctx, info)
	}
	if info.GetAccess() == "" {
		return errors.New("oauth: token has neither code nor access token")
	}
	return s.createAccess(ctx, info)
}

func (s *tokenStore) createCode(ctx context.Context, info oa.TokenInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	ttl := info.GetCodeExpiresIn()
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return s.rdb.Set(ctx, codeKey(info.GetCode()), data, ttl).Err()
}

func (s *tokenStore) createAccess(ctx context.Context, info oa.TokenInfo) error {
	clientID, err := strconv.ParseInt(info.GetClientID(), 10, 64)
	if err != nil {
		return err
	}
	accountID, err := strconv.ParseInt(info.GetUserID(), 10, 64)
	if err != nil {
		return err
	}
	scopes, err := resolveScopes(ctx, s.queries, info.GetScope())
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := s.queries.WithTx(tx)

	sessionID, err := q.CreateOAuthSession(ctx, database.CreateOAuthSessionParams{
		OauthID:   clientID,
		AccountID: accountID,
		Token:     info.GetAccess(),
	})
	if err != nil {
		return err
	}

	if len(scopes) > 0 {
		params := make([]database.AddOAuthSessionScopesParams, len(scopes))
		for i, scope := range scopes {
			params[i] = database.AddOAuthSessionScopesParams{SessionID: sessionID, Scope: scope}
		}
		if _, err := q.AddOAuthSessionScopes(ctx, params); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *tokenStore) GetByCode(ctx context.Context, code string) (oa.TokenInfo, error) {
	data, err := s.rdb.GetDel(ctx, codeKey(code)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var token models.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *tokenStore) RemoveByCode(ctx context.Context, code string) error {
	return s.rdb.Del(ctx, codeKey(code)).Err()
}

func (s *tokenStore) GetByAccess(ctx context.Context, access string) (oa.TokenInfo, error) {
	session, err := s.queries.GetOAuthSession(ctx, access)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	scopes, err := s.queries.GetOAuthSessionScopes(ctx, session.ID)
	if err != nil {
		return nil, err
	}

	token := models.NewToken()
	token.SetClientID(strconv.FormatInt(session.OauthID, 10))
	token.SetUserID(strconv.FormatInt(session.AccountID, 10))
	token.SetScope(strings.Join(scopes, " "))
	token.SetAccess(session.Token)
	token.SetAccessCreateAt(session.Created.Time)
	token.SetAccessExpiresIn(tokenLifetime)
	return token, nil
}

func (s *tokenStore) RemoveByAccess(ctx context.Context, access string) error {
	_, err := s.queries.RemoveOAuthSession(ctx, access)
	return err
}

func (s *tokenStore) GetByRefresh(context.Context, string) (oa.TokenInfo, error) {
	return nil, nil
}

func (s *tokenStore) RemoveByRefresh(context.Context, string) error { return nil }

type clientStore struct {
	queries *database.Queries
}

func (s *clientStore) GetByID(ctx context.Context, id string) (oa.ClientInfo, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, oaerrors.ErrInvalidClient
	}

	row, err := s.queries.GetOAuth(ctx, n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, oaerrors.ErrInvalidClient
		}
		slog.Error("oauth client lookup failed", "err", err)
		return nil, oaerrors.ErrServerError
	}

	return &client{
		id:           id,
		secretHash:   row.Secret,
		redirectURIs: row.RedirectUris,
	}, nil
}

type client struct {
	id           string
	secretHash   string
	redirectURIs []string
}

func (c *client) GetID() string     { return c.id }
func (c *client) GetSecret() string { return c.secretHash }
func (c *client) GetDomain() string { return strings.Join(c.redirectURIs, " ") }
func (c *client) IsPublic() bool    { return false }
func (c *client) GetUserID() string { return "" }

func (c *client) VerifyPassword(secret string) bool {
	return subtle.ConstantTimeCompare([]byte(c.secretHash), []byte(helpers.SHA256(secret))) == 1
}
