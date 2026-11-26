-- name: Get2FA :one
SELECT token FROM tfa WHERE account_id = $1;

-- name: Set2FA :execrows
INSERT INTO tfa (account_id, token)
VALUES (
    $1,
    $2
)
ON CONFLICT (account_id)
DO UPDATE SET token = $2;

-- name: Remove2FA :execrows
DELETE FROM tfa WHERE account_id = $1;