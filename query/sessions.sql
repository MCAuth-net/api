-- name: CreateSession :one
INSERT INTO sessions (account_id, token, ip, agent)
VALUES (
   $1,
   $2,
   $3,
   $4
)
RETURNING expires;

-- name: RemoveSession :execrows
DELETE FROM sessions WHERE token = $1;