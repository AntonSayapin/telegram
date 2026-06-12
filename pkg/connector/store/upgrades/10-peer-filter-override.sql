-- v10 (compatible with v2+): Add manual peer filter override state

CREATE TABLE telegram_peer_filter_override (
    telegram_user_id BIGINT NOT NULL,
    peer_type        TEXT   NOT NULL,
    peer_id          BIGINT NOT NULL,
    state            TEXT   NOT NULL,
    created_at       BIGINT NOT NULL,
    updated_at       BIGINT NOT NULL,

    PRIMARY KEY (telegram_user_id, peer_type, peer_id)
);
