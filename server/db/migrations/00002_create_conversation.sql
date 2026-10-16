-- +goose Up

CREATE TABLE conversation (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind VARCHAR(20) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT conversation_kind_allowed
        CHECK (kind IN ('direct', 'group')),

    CONSTRAINT conversation_created_by_foreign_key
        FOREIGN KEY (created_by)
        REFERENCES identity (id)
        ON DELETE RESTRICT
);

CREATE INDEX conversation_created_by_id_index
    ON conversation (created_by, id DESC);

-- +goose Down

DROP TABLE conversation;
