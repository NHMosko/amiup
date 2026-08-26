-- name: CreateService :one
INSERT INTO services (
	id,
	created_at,
	name,
	url,
	discord_webhook,
	timeout,
	heartbeat,
	strikes,
	was_down,
	when_down,
	strike_counter,
	total_counter,
	down_counter
)
VALUES (
	gen_random_uuid(),
	NOW(),
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8,
	$9,
	$10,
	$11
)
RETURNING *;

-- name: EditService :exec
UPDATE services
SET name = $2,
	url = $3,
	discord_webhook = $4,
	timeout = $5,
	heartbeat = $6,
	strikes = $7
WHERE id = $1;

-- name: UpdateService :exec
UPDATE services
SET down_counter = $2,
	was_down = $3,
	when_down = $4,
	strike_counter = $5,
	total_counter = $6
WHERE id = $1;

-- name: GetServices :many
SELECT * FROM services
ORDER BY created_at DESC;

-- name: GetService :one
SELECT * FROM services
WHERE id = $1;

-- name: DeleteService :exec
DELETE FROM services
WHERE id = $1;
