-- name: CreateOAuth :one
INSERT INTO oauth (name, secret)
VALUES (
    $1,
    $2
)
RETURNING id;

-- name: CreateOAuthSession :one
INSERT INTO oauth_sessions (oauth_id, account_id, token)
VALUES (
    $1,
    $2,
    $3
)
RETURNING id;

-- name: AddOAuthSessionScopes :copyfrom
INSERT INTO oauth_session_scopes (session_id, scope_id)
VALUES (
    $1,
    $2
);

-- name: GetOAuth :one
SELECT * FROM oauth WHERE id = $1;

-- name: GetOAuthFromSecret :one
SELECT * FROM oauth WHERE secret = $1;