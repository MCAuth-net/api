-- +goose up
CREATE TABLE accounts (
    id bigserial PRIMARY KEY,
    email varchar UNIQUE,
    password varchar
);

CREATE TABLE account_uuids (
    uuid uuid PRIMARY KEY,
    account_id bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE
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
    token varchar(32) NOT NULL,
    ip inet NOT NULL,
    protocol_version smallint NOT NULL,
    brand varchar,
    lang varchar
);

-- +goose down
DROP TABLE IF EXISTS
    accounts, account_uuids, sessions, logins
CASCADE;