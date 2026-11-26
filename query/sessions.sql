-- name: CreateSession :one
INSERT INTO sessions (account_id, token, ip, agent, activated)
VALUES (
   $1,
   $2,
   $3,
   $4,
    $5
)
RETURNING expires;

-- name: ActivateSession :execrows
UPDATE sessions SET activated = $2 WHERE token = $1;

-- name: RemoveSession :execrows
DELETE FROM sessions WHERE token = $1;