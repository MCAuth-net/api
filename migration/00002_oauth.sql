-- +goose up
CREATE TABLE scopes (
    id serial PRIMARY KEY,
    name varchar UNIQUE
);

CREATE TABLE oauth (
    id bigserial PRIMARY KEY,
    name varchar UNIQUE,
    secret varchar NOT NULL UNIQUE,
    redirect_uris varchar[] NOT NULL DEFAULT '{}',
    owner bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL
);

CREATE TABLE oauth_sessions (
    id bigserial PRIMARY KEY,
    oauth_id bigint REFERENCES oauth(id) ON DELETE CASCADE NOT NULL,
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE NOT NULL,
    token varchar(64) NOT NULL UNIQUE,
    created timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE oauth_session_scopes (
    session_id bigint REFERENCES oauth_sessions(id) ON DELETE CASCADE NOT NULL,
    scope_id int REFERENCES scopes(id) ON DELETE CASCADE NOT NULL,
    PRIMARY KEY (session_id, scope_id)
);

-- +goose down
DROP TABLE IF EXISTS
    scopes, oauth, oauth_sessions, oauth_session_scopes
CASCADE;