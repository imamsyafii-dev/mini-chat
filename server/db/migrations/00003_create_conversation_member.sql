-- +goose Up

CREATE TABLE conversation_member (
    conversation_id BIGINT NOT NULL,
    identity_id BIGINT NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT conversation_member_primary_key
        PRIMARY KEY (conversation_id, identity_id),

    CONSTRAINT conversation_member_role_allowed
        CHECK (role IN ('owner', 'member')),

    CONSTRAINT conversation_member_conversation_foreign_key
        FOREIGN KEY (conversation_id)
        REFERENCES conversation (id)
        ON DELETE CASCADE,

    CONSTRAINT conversation_member_identity_foreign_key
        FOREIGN KEY (identity_id)
        REFERENCES identity (id)
        ON DELETE CASCADE
);

CREATE INDEX conversation_member_identity_conversation_index
    ON conversation_member (identity_id, conversation_id);

-- +goose Down

DROP TABLE conversation_member;
