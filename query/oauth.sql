-- name: CreateOAuth :one
INSERT INTO oauth (name, secret, owner)
VALUES (
    $1,
    $2,
    $3
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

-- name: GetOAuthWithOwner :one
SELECT * FROM oauth WHERE id = $1 AND owner = $2;

-- name: GetOAuths :many
SELECT * FROM oauth WHERE owner = $1;

-- name: GetScopes :many
SELECT * FROM scopes WHERE name = ANY(sqlc.arg(names)::varchar[]);

-- name: GetOAuthSession :one
SELECT * FROM oauth_sessions WHERE token = $1;

-- name: GetOAuthSessionScopes :many
SELECT s.name
FROM oauth_session_scopes ss
         JOIN scopes s ON s.id = ss.scope_id
WHERE ss.session_id = $1;

-- name: UpdateOAuth :execrows
UPDATE oauth SET name = $2, redirect_uris = $3 WHERE id = $1;

-- name: UpdateOAuthSecretWithOwner :execrows
UPDATE oauth SET secret = $3 WHERE id = $1 AND owner = $2;

-- name: DeleteOAuthWithOwner :execrows
DELETE FROM oauth WHERE id = $1 AND owner = $2;

-- name: RemoveOAuthSession :execrows
DELETE FROM oauth_sessions WHERE token = $1;