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

-- name: UpdateService :exec
UPDATE services
SET strikes = $2,
	was_down = $3,
	when_down = $4,
	strike_counter = $5,
	total_counter = $6,
	down_counter = $7
WHERE id = $1;

-- name: GetServices :many
SELECT * FROM services
ORDER BY created_at DESC;
