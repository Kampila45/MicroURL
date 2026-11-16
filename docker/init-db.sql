CREATE TABLE IF NOT EXISTS links (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    original_url TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,
    click_count BIGINT NOT NULL DEFAULT 0
);

ALTER TABLE links ADD COLUMN IF NOT EXISTS click_count BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_links_slug ON links(slug);

CREATE TABLE IF NOT EXISTS link_click_daily (
    link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    click_date DATE NOT NULL,
    clicks BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (link_id, click_date)
);

CREATE INDEX IF NOT EXISTS idx_link_click_daily_date ON link_click_daily(link_id, click_date);
