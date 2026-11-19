-- name: GetAccountFromSessionToken :one
SELECT a.* FROM accounts a
JOIN sessions s ON s.account_id = a.id
WHERE s.token = $1;