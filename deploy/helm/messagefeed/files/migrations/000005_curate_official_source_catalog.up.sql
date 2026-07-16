-- 重整默认官方源目录：保留各领域少量高质量、可抓取的权威源。

DELETE FROM source_catalog_entries
WHERE source_origin = 'official_seed';

INSERT INTO source_catalog_entries (
    source_key, name, site_url, feed_url, normalized_url, type, category, tags, language, country, official, source_origin, health_status
) VALUES
('openai-news', 'OpenAI News', 'https://openai.com/news/', 'https://openai.com/news/rss.xml', 'https://openai.com/news/rss.xml', 'rss', 'ai', '["ai","official","research"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('deepmind-blog', 'Google DeepMind Blog', 'https://deepmind.google/blog/', 'https://deepmind.google/blog/rss.xml', 'https://deepmind.google/blog/rss.xml', 'rss', 'ai', '["ai","research"]'::jsonb, 'en', 'GB', true, 'official_seed', 'healthy'),
('arxiv-cs-ai', 'arXiv CS.AI', 'https://arxiv.org/list/cs.AI/recent', 'https://rss.arxiv.org/rss/cs.AI', 'https://rss.arxiv.org/rss/cs.AI', 'rss', 'ai', '["ai","papers"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('github-blog', 'GitHub Blog', 'https://github.blog/', 'https://github.blog/feed/', 'https://github.blog/feed/', 'rss', 'developer', '["developer","github"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('go-blog', 'Go Blog', 'https://go.dev/blog/', 'https://go.dev/blog/feed.atom', 'https://go.dev/blog/feed.atom', 'atom', 'developer', '["go","programming"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('rust-blog', 'Rust Blog', 'https://blog.rust-lang.org/', 'https://blog.rust-lang.org/feed.xml', 'https://blog.rust-lang.org/feed.xml', 'rss', 'developer', '["rust","programming"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('kubernetes-blog', 'Kubernetes Blog', 'https://kubernetes.io/blog/', 'https://kubernetes.io/feed.xml', 'https://kubernetes.io/feed.xml', 'rss', 'cloud', '["kubernetes","cloud-native"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('cloudflare-blog', 'Cloudflare Blog', 'https://blog.cloudflare.com/', 'https://blog.cloudflare.com/rss/', 'https://blog.cloudflare.com/rss/', 'rss', 'cloud', '["cloud","network","security"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('linux-foundation-blog', 'Linux Foundation Blog', 'https://www.linuxfoundation.org/blog', 'https://www.linuxfoundation.org/blog/rss.xml', 'https://www.linuxfoundation.org/blog/rss.xml', 'rss', 'cloud', '["linux","open-source"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('google-security-blog', 'Google Online Security Blog', 'https://security.googleblog.com/', 'https://security.googleblog.com/feeds/posts/default?alt=rss', 'https://security.googleblog.com/feeds/posts/default?alt=rss', 'rss', 'security', '["security","google"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('microsoft-security-blog', 'Microsoft Security Blog', 'https://www.microsoft.com/en-us/security/blog/', 'https://www.microsoft.com/en-us/security/blog/feed/', 'https://www.microsoft.com/en-us/security/blog/feed/', 'rss', 'security', '["security","microsoft"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('nasa-news', 'NASA News Releases', 'https://www.nasa.gov/news-release/', 'https://www.nasa.gov/news-release/feed/', 'https://www.nasa.gov/news-release/feed/', 'rss', 'science', '["science","space"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('esa-news', 'ESA News', 'https://www.esa.int/Newsroom', 'https://www.esa.int/rssfeed/Our_Activities', 'https://www.esa.int/rssfeed/Our_Activities', 'rss', 'science', '["science","space"]'::jsonb, 'en', 'EU', true, 'official_seed', 'healthy'),
('nature-news', 'Nature', 'https://www.nature.com/nature/', 'https://www.nature.com/nature.rss', 'https://www.nature.com/nature.rss', 'rss', 'science', '["science","journal"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('who-news', 'WHO News', 'https://www.who.int/news', 'https://www.who.int/rss-feeds/news-english.xml', 'https://www.who.int/rss-feeds/news-english.xml', 'rss', 'health', '["health","global"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('cdc-newsroom', 'CDC Newsroom', 'https://www.cdc.gov/media/index.html', 'https://tools.cdc.gov/api/v2/resources/media/132608.rss', 'https://tools.cdc.gov/api/v2/resources/media/132608.rss', 'rss', 'health', '["health","public-health"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('federal-reserve-press', 'Federal Reserve Press', 'https://www.federalreserve.gov/newsevents/pressreleases.htm', 'https://www.federalreserve.gov/feeds/press_all.xml', 'https://www.federalreserve.gov/feeds/press_all.xml', 'rss', 'finance', '["finance","central-bank"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('ecb-press', 'ECB Press', 'https://www.ecb.europa.eu/press/html/index.en.html', 'https://www.ecb.europa.eu/rss/press.html', 'https://www.ecb.europa.eu/rss/press.html', 'rss', 'finance', '["finance","central-bank"]'::jsonb, 'en', 'EU', true, 'official_seed', 'healthy'),
('un-news', 'UN News', 'https://news.un.org/en/', 'https://news.un.org/feed/subscribe/en/news/all/rss.xml', 'https://news.un.org/feed/subscribe/en/news/all/rss.xml', 'rss', 'world', '["world","news"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('bbc-world', 'BBC World News', 'https://www.bbc.com/news/world', 'https://feeds.bbci.co.uk/news/world/rss.xml', 'https://feeds.bbci.co.uk/news/world/rss.xml', 'rss', 'world', '["world","news"]'::jsonb, 'en', 'GB', true, 'official_seed', 'healthy'),
('ap-top-news', 'AP Top News', 'https://apnews.com/hub/ap-top-news', 'https://apnews.com/hub/ap-top-news?output=rss', 'https://apnews.com/hub/ap-top-news?output=rss', 'rss', 'world', '["news","wire"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('noaa-news', 'NOAA News', 'https://www.noaa.gov/news', 'https://www.noaa.gov/rss.xml', 'https://www.noaa.gov/rss.xml', 'rss', 'climate', '["climate","weather","science"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('ipcc-news', 'IPCC News', 'https://www.ipcc.ch/', 'https://www.ipcc.ch/feed/', 'https://www.ipcc.ch/feed/', 'rss', 'climate', '["climate","science"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('our-world-in-data', 'Our World in Data', 'https://ourworldindata.org/', 'https://ourworldindata.org/atom.xml', 'https://ourworldindata.org/atom.xml', 'atom', 'data', '["data","research","global"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('w3c-news', 'W3C News', 'https://www.w3.org/news/', 'https://www.w3.org/news/feed/', 'https://www.w3.org/news/feed/', 'rss', 'standards', '["web","standards"]'::jsonb, 'en', '', true, 'official_seed', 'healthy'),
('mit-technology-review', 'MIT Technology Review', 'https://www.technologyreview.com/', 'https://www.technologyreview.com/feed/', 'https://www.technologyreview.com/feed/', 'rss', 'technology', '["technology","analysis"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy'),
('smithsonian-latest', 'Smithsonian Magazine', 'https://www.smithsonianmag.com/', 'https://www.smithsonianmag.com/rss/latest_articles/', 'https://www.smithsonianmag.com/rss/latest_articles/', 'rss', 'culture', '["culture","history","science"]'::jsonb, 'en', 'US', true, 'official_seed', 'healthy')
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
