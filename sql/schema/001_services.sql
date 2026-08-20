-- +goose Up
CREATE TABLE services (
	id UUID PRIMARY KEY,
	created_at TIMESTAMP NOT NULL,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	timeout INT NOT NULL,
	heartbeat INT NOT NULL,
	strikes INT NOT NULL,
	was_down BOOLEAN,
	when_down TIMESTAMP,
	strike_counter INT,
	total_counter INT,
	down_counter INT
);

-- +goose Down	
DROP TABLE services;
