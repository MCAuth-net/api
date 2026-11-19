-- name: GetAccounts :many
SELECT * FROM accounts ORDER BY id;

-- name: GetAccountByUUID :one
SELECT account_id FROM account_uuids WHERE uuid = $1;

-- name: GetUUIDsByAccountID :many
SELECT uuid FROM account_uuids WHERE account_id = $1;