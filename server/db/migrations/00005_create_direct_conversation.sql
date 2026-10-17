-- +goose Up

CREATE TABLE direct_conversation (
    conversation_id BIGINT NOT NULL,
    identity_low_id BIGINT NOT NULL,
    identity_high_id BIGINT NOT NULL,

    CONSTRAINT direct_conversation_primary_key
        PRIMARY KEY (conversation_id),

    CONSTRAINT direct_conversation_identity_pair_canonical
        CHECK (identity_low_id < identity_high_id),

    CONSTRAINT direct_conversation_identity_pair_unique
        UNIQUE (identity_low_id, identity_high_id),

    CONSTRAINT direct_conversation_conversation_foreign_key
        FOREIGN KEY (conversation_id)
        REFERENCES conversation (id)
        ON DELETE CASCADE,

    CONSTRAINT direct_conversation_identity_low_foreign_key
        FOREIGN KEY (identity_low_id)
        REFERENCES identity (id)
        ON DELETE RESTRICT,

    CONSTRAINT direct_conversation_identity_high_foreign_key
        FOREIGN KEY (identity_high_id)
        REFERENCES identity (id)
        ON DELETE RESTRICT
);

-- +goose Down

DROP TABLE direct_conversation;
