CREATE TABLE anime_news (
    id VARCHAR(40) PRIMARY KEY,
    anime_id VARCHAR(36) NOT NULL,
    mal_id INT NULL,
    title VARCHAR(512) NOT NULL,
    summary TEXT NULL,
    category VARCHAR(32) NOT NULL,
    source_url TEXT NULL,
    source_name VARCHAR(255) NULL,
    published_date DATE NULL,
    episode_number INT NULL,
    title_slug VARCHAR(255) NULL,
    researched_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_news_anime (anime_id),
    UNIQUE INDEX idx_news_dedupe (anime_id, published_date, title_slug)
);
