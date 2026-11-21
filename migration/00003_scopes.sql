-- +goose up
INSERT INTO scopes (name)
VALUES
    ('email'),
    ('alts'),
    ('ipinfo');

-- +goose down
DELETE FROM scopes WHERE name IN (
    'email',
    'alts',
    'ipinfo'
);