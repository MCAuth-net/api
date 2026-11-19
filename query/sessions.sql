-- name: GetAccountFromSessionToken :one
SELECT a.* FROM accounts a
JOIN sessions s ON s.account_id = a.id
WHERE s.token = $1;

-- name: CreateSession :one
INSERT INTO sessions (account_id, token, ip, agent)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING expires;