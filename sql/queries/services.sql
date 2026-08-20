-- name: CreateService :one
INSERT INTO services (
	id,
	created_at,
	name,
	url,
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
	$10
)
RETURNING *;
