-- v0 -> v10 (compatible with v2+): Latest revision

CREATE TABLE telegram_user_state (
    user_id BIGINT NOT NULL PRIMARY KEY,
    pts     BIGINT NOT NULL,
    qts     BIGINT NOT NULL,
    date    BIGINT NOT NULL,
    seq     BIGINT NOT NULL
);

CREATE TABLE telegram_channel_state (
    user_id    BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    pts        BIGINT NOT NULL,

    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE telegram_access_hash (
    user_id     BIGINT NOT NULL,
    entity_type TEXT   NOT NULL,
    entity_id   BIGINT NOT NULL,
    access_hash BIGINT NOT NULL,

    PRIMARY KEY (user_id, entity_type, entity_id)
);

CREATE TABLE telegram_username (
    username    TEXT   NOT NULL,
    entity_type TEXT   NOT NULL,
    entity_id   BIGINT NOT NULL,

    PRIMARY KEY (username)
);

CREATE INDEX telegram_username_entity_idx ON telegram_username (entity_id);
CREATE INDEX telegram_username_username_idx ON telegram_username (LOWER(username));

CREATE TABLE telegram_phone_number (
    phone_number  TEXT   NOT NULL,
    entity_id     BIGINT NOT NULL,

    PRIMARY KEY (phone_number)
);

CREATE INDEX telegram_phone_number_entity_idx ON telegram_phone_number (entity_id);

CREATE TABLE telegram_file (
    id        TEXT PRIMARY KEY,
    mxc       TEXT NOT NULL,
    mime_type TEXT,
    size      BIGINT,
    width     INTEGER,
    height    INTEGER,
    timestamp BIGINT
);

CREATE INDEX telegram_file_mxc_idx ON telegram_file (mxc);

CREATE TABLE telegram_topic (
    channel_id BIGINT NOT NULL,
    topic_id   BIGINT NOT NULL,

    PRIMARY KEY (channel_id, topic_id)
);

CREATE TABLE telegram_peer_filter_override (
    telegram_user_id BIGINT NOT NULL,
    peer_type        TEXT   NOT NULL,
    peer_id          BIGINT NOT NULL,
    state            TEXT   NOT NULL,
    created_at       BIGINT NOT NULL,
    updated_at       BIGINT NOT NULL,

    PRIMARY KEY (telegram_user_id, peer_type, peer_id)
);
