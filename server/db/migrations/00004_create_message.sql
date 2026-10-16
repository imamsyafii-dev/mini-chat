-- +goose Up

CREATE TABLE message (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    conversation_id BIGINT NOT NULL,
    sender_id BIGINT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT message_body_length_allowed
        CHECK (
            CHAR_LENGTH(BTRIM(body)) BETWEEN 1 AND 4000
        ),

    CONSTRAINT message_conversation_foreign_key
        FOREIGN KEY (conversation_id)
        REFERENCES conversation (id)
        ON DELETE CASCADE,

    CONSTRAINT message_sender_foreign_key
        FOREIGN KEY (sender_id)
        REFERENCES identity (id)
        ON DELETE RESTRICT
);

CREATE INDEX message_conversation_id_id_index
    ON message (conversation_id, id DESC);

CREATE INDEX message_sender_id_index
    ON message (sender_id);

-- +goose Down

DROP TABLE message;
