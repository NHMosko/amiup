-- +goose Up
CREATE TABLE services (
	id UUID PRIMARY KEY,
	created_at TIMESTAMP NOT NULL,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	discord_webhook TEXT NOT NULL,
	timeout INT NOT NULL,
	heartbeat INT NOT NULL,
	strikes INT NOT NULL,
	was_down BOOLEAN NOT NULL DEFAULT FALSE,
	when_down TIMESTAMP,
	strike_counter INT NOT NULL DEFAULT 0,
	total_counter INT NOT NULL DEFAULT 0,
	down_counter INT NOT NULL DEFAULT 0
);

-- +goose Down	
DROP TABLE services;
