CREATE TABLE anime_fanart (
    id VARCHAR(40) PRIMARY KEY,
    anime_id VARCHAR(36) NOT NULL,
    image_url TEXT NOT NULL,
    source_url TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_fanart_anime (anime_id)
);
