-- name: GetAccounts :many
SELECT * FROM accounts ORDER BY id;

-- name: GetAccountByUUID :one
SELECT account_id FROM account_uuids WHERE uuid = $1;