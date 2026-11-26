-- name: GetAccounts :many
SELECT * FROM accounts ORDER BY id;

-- name: GetAccount :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountWithUUID :one
SELECT a.*, u.uuid AS uuid
FROM accounts a
    LEFT JOIN account_uuids u ON u.account_id = a.id AND u.selected = TRUE
WHERE a.id = $1;

-- name: GetAccountByUUID :one
SELECT a.*
FROM account_uuids u
    JOIN accounts a ON a.id = u.account_id
WHERE u.uuid = $1;

-- name: GetAccountByEmail :one
SELECT * FROM accounts WHERE email = $1;

-- name: GetAccountFromSessionToken :one
SELECT a.*, s.expires AS expires, s.activated, s.id AS session_id, u.uuid AS uuid
FROM accounts a
    LEFT JOIN account_uuids u ON u.account_id = a.id AND u.selected = TRUE
    JOIN sessions s ON s.account_id = a.id
WHERE s.token = $1;

-- name: GetUUIDsByAccountID :many
SELECT uuid FROM account_uuids WHERE account_id = $1;

-- name: GetPfp :one
SELECT pfp, mime FROM account_pfps WHERE account_id = $1;

-- name: ChangeEmail :execrows
UPDATE accounts SET email = $2 WHERE id = $1;

-- name: ChangePassword :execrows
UPDATE accounts SET password = $2 WHERE id = $1;

-- name: ChangeName :execrows
UPDATE accounts SET name = $2 WHERE id = $1;

-- name: ChangeColor :execrows
UPDATE accounts SET color = $2 WHERE id = $1;

-- name: ChangePfp :execrows
INSERT INTO account_pfps (account_id, pfp, mime)
VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (account_id)
DO UPDATE SET pfp = excluded.pfp;

-- name: Change2FAMethods :execrows
UPDATE accounts SET tfa_methods = $2 WHERE id = $1;

-- name: SetPrimaryUUID :execrows
WITH unset_previous AS (
    UPDATE account_uuids
        SET selected = FALSE
        WHERE account_id = $2 AND selected = TRUE
)
UPDATE account_uuids
SET selected = TRUE
WHERE account_uuids.uuid = $1
    AND account_uuids.account_id = $2;

-- name: HasPfp :one
SELECT EXISTS (
    SELECT 1
    FROM account_pfps
    WHERE account_id = $1
);

-- name: DeleteAccount :execrows
DELETE FROM accounts WHERE id = $1;