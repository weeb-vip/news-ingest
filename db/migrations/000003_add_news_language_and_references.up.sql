ALTER TABLE anime_news
    ADD COLUMN language VARCHAR(8) NULL AFTER source_name,
    ADD COLUMN reference_links JSON NULL AFTER language;
