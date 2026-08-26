-- name: CreateRequisition :one
INSERT INTO requisitions (
	id,
	service_id,
	request_time,
	status,
	response_body,
	duration
)
VALUES (
	gen_random_uuid(),
	$1,
	$2,
	$3,
	$4,
	$5
)
RETURNING *;

-- name: GetFormattedRequisitions :many
SELECT (
		SELECT name 
		FROM services
		WHERE service_id = id
	), 
	request_time, 
	status, 
	duration, 
	LENGTH(response_body) 
FROM requisitions;

-- name: GetRequisitions :many
SELECT * FROM requisitions
ORDER BY request_time DESC;

-- name: GetRequisitionsForService :many
SELECT * FROM requisitions
WHERE service_id = $1
ORDER BY request_time DESC;
