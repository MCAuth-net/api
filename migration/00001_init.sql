-- +goose up
CREATE TABLE accounts (
    id bigserial PRIMARY KEY,
    uuid uuid[] NOT NULL DEFAULT '{}',
    email varchar UNIQUE,
    password varchar
);

CREATE TABLE sessions (
    id bigserial PRIMARY KEY,
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL,
    token varchar NOT NULL UNIQUE,
    ip inet NOT NULL,
    agent varchar NOT NULL
);

CREATE TABLE logins (
    id bigserial PRIMARY KEY,
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL,
    uuid uuid NOT NULL,
    ip inet NOT NULL,
    protocol_version smallint NOT NULL,
    brand varchar,
    lang varchar
);

-- +goose down
DROP TABLE IF EXISTS
    accounts, sessions, logins
CASCADE;