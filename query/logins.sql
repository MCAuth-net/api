-- name: GetUUIDFromLoginToken :one
SELECT uuid FROM logins WHERE token = $1;