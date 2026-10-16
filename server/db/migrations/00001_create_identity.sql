-- +goose Up

CREATE TABLE identity (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username VARCHAR(40) NOT NULL,
    password_hash TEXT NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT identity_username_not_blank
        CHECK (BTRIM(username) <> ''),

    CONSTRAINT identity_username_lowercase
        CHECK (username = LOWER(username)),

    CONSTRAINT identity_username_unique
        UNIQUE (username),

    CONSTRAINT identity_password_hash_not_blank
        CHECK (BTRIM(password_hash) <> ''),

    CONSTRAINT identity_display_name_not_blank
        CHECK (BTRIM(display_name) <> '')
);

-- +goose Down

DROP TABLE identity;
