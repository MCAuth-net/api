-- name: GetAccounts :many
SELECT * FROM accounts ORDER BY id;

-- name: GetAccountByUUID :one
SELECT account_id FROM account_uuids WHERE uuid = $1;

-- name: GetAccountByEmail :one
SELECT id, password FROM accounts WHERE email = $1;

-- name: GetUUIDsByAccountID :many
SELECT uuid FROM account_uuids WHERE account_id = $1;

-- name: SetEmail :execrows
UPDATE accounts SET email = $2 WHERE id = $1;