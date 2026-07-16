-- messageFeed 官方源目录与导入任务迁移
-- 文件名：000003_add_source_catalog_imports.up.sql

CREATE TABLE IF NOT EXISTS source_catalog_entries (
    id BIGSERIAL PRIMARY KEY,
    source_key VARCHAR(160) NOT NULL,
    name VARCHAR(255) NOT NULL,
    site_url TEXT,
    feed_url TEXT NOT NULL,
    normalized_url TEXT NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'rss',
    category VARCHAR(80) NOT NULL DEFAULT 'general',
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    language VARCHAR(16) NOT NULL DEFAULT 'en',
    country VARCHAR(16) NOT NULL DEFAULT '',
    official BOOLEAN NOT NULL DEFAULT TRUE,
    source_origin VARCHAR(80) NOT NULL DEFAULT 'official_seed',
    health_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
    last_checked_at TIMESTAMP WITH TIME ZONE,
    last_check_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_source_catalog_entries_type CHECK (type IN ('rss', 'atom', 'json_feed')),
    CONSTRAINT chk_source_catalog_entries_health CHECK (health_status IN ('healthy', 'degraded', 'unreachable', 'unknown')),
    CONSTRAINT uq_source_catalog_entries_origin_key UNIQUE (source_origin, source_key),
    CONSTRAINT uq_source_catalog_entries_normalized_url UNIQUE (normalized_url)
);

CREATE INDEX IF NOT EXISTS idx_source_catalog_entries_category ON source_catalog_entries(category);
CREATE INDEX IF NOT EXISTS idx_source_catalog_entries_health ON source_catalog_entries(health_status);
CREATE INDEX IF NOT EXISTS idx_source_catalog_entries_name ON source_catalog_entries(name);

DROP TRIGGER IF EXISTS update_source_catalog_entries_updated_at ON source_catalog_entries;
CREATE TRIGGER update_source_catalog_entries_updated_at
    BEFORE UPDATE ON source_catalog_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS source_import_jobs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    import_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    requested_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    error_details JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_source_import_jobs_type CHECK (import_type IN ('catalog', 'urls', 'opml')),
    CONSTRAINT chk_source_import_jobs_status CHECK (status IN ('completed', 'partial', 'failed')),
    CONSTRAINT chk_source_import_jobs_counts CHECK (
        requested_count >= 0 AND success_count >= 0 AND failure_count >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_source_import_jobs_user_created ON source_import_jobs(user_id, created_at DESC);

DROP TRIGGER IF EXISTS update_source_import_jobs_updated_at ON source_import_jobs;
CREATE TRIGGER update_source_import_jobs_updated_at
    BEFORE UPDATE ON source_import_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

INSERT INTO source_catalog_entries (
    source_key, name, site_url, feed_url, normalized_url, type, category, tags, language, country, official, source_origin, health_status
) VALUES
('openai-news', 'OpenAI News', 'https://openai.com/news/', 'https://openai.com/news/rss.xml', 'https://openai.com/news/rss.xml', 'rss', 'ai', '["ai","official"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('deepmind-blog', 'DeepMind Blog', 'https://deepmind.google/blog/', 'https://deepmind.google/blog/rss.xml', 'https://deepmind.google/blog/rss.xml', 'rss', 'ai', '["ai","research"]'::jsonb, 'en', 'GB', true, 'official_seed', 'healthy'),
('github-blog', 'GitHub Blog', 'https://github.blog/', 'https://github.blog/feed/', 'https://github.blog/feed/', 'rss', 'developer', '["developer","github"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('github-changelog', 'GitHub Changelog', 'https://github.blog/changelog/', 'https://github.blog/changelog/feed/', 'https://github.blog/changelog/feed/', 'rss', 'developer', '["developer","changelog"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('go-blog', 'Go Blog', 'https://go.dev/blog/', 'https://go.dev/blog/feed.atom', 'https://go.dev/blog/feed.atom', 'atom', 'developer', '["go","programming"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('kubernetes-blog', 'Kubernetes Blog', 'https://kubernetes.io/blog/', 'https://kubernetes.io/feed.xml', 'https://kubernetes.io/feed.xml', 'rss', 'cloud', '["kubernetes","cloud-native"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('cncf-blog', 'CNCF Blog', 'https://www.cncf.io/blog/', 'https://www.cncf.io/feed/', 'https://www.cncf.io/feed/', 'rss', 'cloud', '["cloud-native","cncf"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('cloudflare-blog', 'Cloudflare Blog', 'https://blog.cloudflare.com/', 'https://blog.cloudflare.com/rss/', 'https://blog.cloudflare.com/rss/', 'rss', 'cloud', '["cloud","security","network"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('aws-news-blog', 'AWS News Blog', 'https://aws.amazon.com/blogs/aws/', 'https://aws.amazon.com/blogs/aws/feed/', 'https://aws.amazon.com/blogs/aws/feed/', 'rss', 'cloud', '["aws","cloud"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('aws-whats-new', 'AWS What''s New', 'https://aws.amazon.com/about-aws/whats-new/', 'https://aws.amazon.com/about-aws/whats-new/recent/feed/', 'https://aws.amazon.com/about-aws/whats-new/recent/feed/', 'rss', 'cloud', '["aws","changelog"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('rust-blog', 'Rust Blog', 'https://blog.rust-lang.org/', 'https://blog.rust-lang.org/feed.xml', 'https://blog.rust-lang.org/feed.xml', 'rss', 'developer', '["rust","programming"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('python-blog', 'Python Blog', 'https://blog.python.org/', 'https://blog.python.org/feeds/posts/default?alt=rss', 'https://blog.python.org/feeds/posts/default?alt=rss', 'rss', 'developer', '["python","programming"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('django-news', 'Django News', 'https://www.djangoproject.com/weblog/', 'https://www.djangoproject.com/rss/weblog/', 'https://www.djangoproject.com/rss/weblog/', 'rss', 'developer', '["python","django"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('nodejs-blog', 'Node.js Blog', 'https://nodejs.org/en/blog/', 'https://nodejs.org/en/feed/blog.xml', 'https://nodejs.org/en/feed/blog.xml', 'rss', 'developer', '["nodejs","javascript"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('typescript-blog', 'TypeScript Blog', 'https://devblogs.microsoft.com/typescript/', 'https://devblogs.microsoft.com/typescript/feed/', 'https://devblogs.microsoft.com/typescript/feed/', 'rss', 'developer', '["typescript","javascript"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('react-blog', 'React Blog', 'https://react.dev/blog', 'https://react.dev/rss.xml', 'https://react.dev/rss.xml', 'rss', 'developer', '["react","frontend"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('vue-blog', 'Vue Blog', 'https://blog.vuejs.org/', 'https://blog.vuejs.org/feed.rss', 'https://blog.vuejs.org/feed.rss', 'rss', 'developer', '["vue","frontend"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('chrome-developers', 'Chrome Developers', 'https://developer.chrome.com/blog/', 'https://developer.chrome.com/static/blog/feed.xml', 'https://developer.chrome.com/static/blog/feed.xml', 'rss', 'developer', '["chrome","web"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('mozilla-blog', 'Mozilla Blog', 'https://blog.mozilla.org/', 'https://blog.mozilla.org/en/feed/', 'https://blog.mozilla.org/en/feed/', 'rss', 'developer', '["mozilla","web"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('nvidia-blog', 'NVIDIA Blog', 'https://blogs.nvidia.com/', 'https://blogs.nvidia.com/feed/', 'https://blogs.nvidia.com/feed/', 'rss', 'ai', '["ai","hardware"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('apple-newsroom', 'Apple Newsroom', 'https://www.apple.com/newsroom/', 'https://www.apple.com/newsroom/rss-feed.rss', 'https://www.apple.com/newsroom/rss-feed.rss', 'rss', 'technology', '["apple","technology"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('apple-developer-news', 'Apple Developer News', 'https://developer.apple.com/news/', 'https://developer.apple.com/news/rss/news.rss', 'https://developer.apple.com/news/rss/news.rss', 'rss', 'developer', '["apple","developer"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('android-developers', 'Android Developers', 'https://android-developers.googleblog.com/', 'https://android-developers.googleblog.com/feeds/posts/default?alt=rss', 'https://android-developers.googleblog.com/feeds/posts/default?alt=rss', 'rss', 'developer', '["android","mobile"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('gitlab-blog', 'GitLab Blog', 'https://about.gitlab.com/blog/', 'https://about.gitlab.com/atom.xml', 'https://about.gitlab.com/atom.xml', 'atom', 'developer', '["gitlab","devops"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('grafana-blog', 'Grafana Blog', 'https://grafana.com/blog/', 'https://grafana.com/blog/index.xml', 'https://grafana.com/blog/index.xml', 'rss', 'observability', '["grafana","observability"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('elastic-blog', 'Elastic Blog', 'https://www.elastic.co/blog/', 'https://www.elastic.co/blog/feed', 'https://www.elastic.co/blog/feed', 'rss', 'observability', '["elastic","search"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('mongodb-blog', 'MongoDB Blog', 'https://www.mongodb.com/company/blog', 'https://www.mongodb.com/company/blog/rss', 'https://www.mongodb.com/company/blog/rss', 'rss', 'database', '["mongodb","database"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('slack-engineering', 'Slack Engineering', 'https://slack.engineering/', 'https://slack.engineering/feed/', 'https://slack.engineering/feed/', 'rss', 'engineering', '["engineering","saas"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('meta-engineering', 'Meta Engineering', 'https://engineering.fb.com/', 'https://engineering.fb.com/feed/', 'https://engineering.fb.com/feed/', 'rss', 'engineering', '["engineering","meta"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('stripe-blog', 'Stripe Blog', 'https://stripe.com/blog', 'https://stripe.com/blog/feed.rss', 'https://stripe.com/blog/feed.rss', 'rss', 'technology', '["stripe","fintech"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('vercel-blog', 'Vercel Blog', 'https://vercel.com/blog', 'https://vercel.com/atom', 'https://vercel.com/atom', 'atom', 'developer', '["frontend","vercel"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('nextjs-blog', 'Next.js Blog', 'https://nextjs.org/blog', 'https://nextjs.org/feed.xml', 'https://nextjs.org/feed.xml', 'rss', 'developer', '["nextjs","frontend"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('tailwind-blog', 'Tailwind CSS Blog', 'https://tailwindcss.com/blog', 'https://tailwindcss.com/feeds/feed.xml', 'https://tailwindcss.com/feeds/feed.xml', 'rss', 'developer', '["css","frontend"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('svelte-blog', 'Svelte Blog', 'https://svelte.dev/blog', 'https://svelte.dev/blog/rss.xml', 'https://svelte.dev/blog/rss.xml', 'rss', 'developer', '["svelte","frontend"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('mdn-blog', 'MDN Blog', 'https://developer.mozilla.org/en-US/blog/', 'https://developer.mozilla.org/en-US/blog/rss.xml', 'https://developer.mozilla.org/en-US/blog/rss.xml', 'rss', 'developer', '["web","documentation"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('wordpress-news', 'WordPress News', 'https://wordpress.org/news/', 'https://wordpress.org/news/feed/', 'https://wordpress.org/news/feed/', 'rss', 'technology', '["wordpress","cms"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('redis-blog', 'Redis Blog', 'https://redis.io/blog/', 'https://redis.io/blog/feed/', 'https://redis.io/blog/feed/', 'rss', 'database', '["redis","database"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('hashicorp-blog', 'HashiCorp Blog', 'https://www.hashicorp.com/blog/', 'https://www.hashicorp.com/blog/feed.xml', 'https://www.hashicorp.com/blog/feed.xml', 'rss', 'cloud', '["infrastructure","devops"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('berkeley-ai-research', 'Berkeley AI Research', 'https://bair.berkeley.edu/blog/', 'https://bair.berkeley.edu/blog/feed.xml', 'https://bair.berkeley.edu/blog/feed.xml', 'rss', 'ai', '["ai","research"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('arxiv-cs-ai', 'arXiv CS.AI', 'https://arxiv.org/list/cs.AI/recent', 'https://rss.arxiv.org/rss/cs.AI', 'https://rss.arxiv.org/rss/cs.AI', 'rss', 'ai', '["ai","papers"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('sec-press', 'SEC Press Releases', 'https://www.sec.gov/news/pressreleases', 'https://www.sec.gov/news/pressreleases.rss', 'https://www.sec.gov/news/pressreleases.rss', 'rss', 'finance', '["finance","regulation"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('federal-reserve-press', 'Federal Reserve Press', 'https://www.federalreserve.gov/newsevents/pressreleases.htm', 'https://www.federalreserve.gov/feeds/press_all.xml', 'https://www.federalreserve.gov/feeds/press_all.xml', 'rss', 'finance', '["finance","central-bank"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('ecb-press', 'ECB Press', 'https://www.ecb.europa.eu/press/html/index.en.html', 'https://www.ecb.europa.eu/rss/press.html', 'https://www.ecb.europa.eu/rss/press.html', 'rss', 'finance', '["finance","central-bank"]'::jsonb, 'en', 'EU', true, 'official_seed', 'healthy'),
('bbc-news', 'BBC News', 'https://www.bbc.com/news', 'https://feeds.bbci.co.uk/news/rss.xml', 'https://feeds.bbci.co.uk/news/rss.xml', 'rss', 'news', '["news"]'::jsonb, 'en', 'GB', true, 'official_seed', 'healthy'),
('nyt-technology', 'NYT Technology', 'https://www.nytimes.com/section/technology', 'https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml', 'https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml', 'rss', 'technology', '["technology","news"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('npr-news', 'NPR News', 'https://www.npr.org/sections/news/', 'https://feeds.npr.org/1001/rss.xml', 'https://feeds.npr.org/1001/rss.xml', 'rss', 'news', '["news"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('guardian-world', 'The Guardian World', 'https://www.theguardian.com/world', 'https://www.theguardian.com/world/rss', 'https://www.theguardian.com/world/rss', 'rss', 'news', '["world","news"]'::jsonb, 'en', 'GB', true, 'official_seed', 'healthy'),
('wsj-markets', 'WSJ Markets', 'https://www.wsj.com/news/markets', 'https://feeds.a.dj.com/rss/RSSMarketsMain.xml', 'https://feeds.a.dj.com/rss/RSSMarketsMain.xml', 'rss', 'finance', '["finance","markets"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('noaa-news', 'NOAA News', 'https://www.noaa.gov/news', 'https://www.noaa.gov/rss.xml', 'https://www.noaa.gov/rss.xml', 'rss', 'science', '["science","weather"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('who-news', 'WHO News', 'https://www.who.int/news', 'https://www.who.int/rss-feeds/news-english.xml', 'https://www.who.int/rss-feeds/news-english.xml', 'rss', 'health', '["health","global"]'::jsonb, 'en', '', true, 'official_seed', 'healthy')
ON CONFLICT (source_origin, source_key) DO UPDATE SET
    name = EXCLUDED.name,
    site_url = EXCLUDED.site_url,
    feed_url = EXCLUDED.feed_url,
    normalized_url = EXCLUDED.normalized_url,
    type = EXCLUDED.type,
    category = EXCLUDED.category,
    tags = EXCLUDED.tags,
    language = EXCLUDED.language,
    country = EXCLUDED.country,
    official = EXCLUDED.official,
    health_status = EXCLUDED.health_status;
