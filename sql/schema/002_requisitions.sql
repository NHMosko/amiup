-- +goose Up
CREATE TABLE requisitions (
	id UUID PRIMARY KEY,
	service_id UUID REFERENCES services(id) ON DELETE CASCADE NOT NULL,
	request_time TIMESTAMP NOT NULL,
	status INT NOT NULL,
	response_body BYTEA NOT NULL,
	duration INT NOT NULL
);

-- +goose Down	
DROP TABLE requisitions;
