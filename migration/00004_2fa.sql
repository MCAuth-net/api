-- +goose up
CREATE TABLE tfa (
    account_id bigint REFERENCES accounts(id) ON DELETE CASCADE PRIMARY KEY,
    token varchar(32) NOT NULL
);

-- +goose down
DROP TABLE IF EXISTS
    tfa
CASCADE;