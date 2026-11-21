-- +goose up
CREATE TABLE accounts (
    id bigserial PRIMARY KEY,
    email varchar UNIQUE,
    password varchar,
    name varchar
);

CREATE TABLE account_uuids (
    uuid uuid PRIMARY KEY,
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL,
    selected boolean NOT NULL DEFAULT FALSE
);

CREATE UNIQUE INDEX unique_selected_uuid_per_account
    ON account_uuids (account_id)
    WHERE selected = TRUE;

CREATE TABLE sessions (
    id bigserial PRIMARY KEY,
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL,
    token varchar(64) NOT NULL UNIQUE,
    ip inet NOT NULL,
    agent varchar NOT NULL,
    expires timestamptz NOT NULL DEFAULT NOW() + INTERVAL '30 days'
);

CREATE TABLE logins (
    id bigserial PRIMARY KEY,
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL,
    uuid uuid NOT NULL,
    token varchar(64) NOT NULL,
    ip inet NOT NULL,
    protocol_version smallint NOT NULL,
    brand varchar,
    lang varchar
);

-- +goose down
DROP TABLE IF EXISTS
    accounts, account_uuids, sessions, logins
CASCADE;