-- Helm 打包副本；源迁移位于项目 migrations 目录。
-- 增补中文官方源目录：覆盖新闻、科技、开发、商业、文化等中文内容。

INSERT INTO source_catalog_entries (
    source_key, name, site_url, feed_url, normalized_url, type, category, tags, language, country, official, source_origin, health_status
) VALUES
('bbc-zhongwen', 'BBC 中文', 'https://www.bbc.com/zhongwen/simp', 'https://feeds.bbci.co.uk/zhongwen/simp/rss.xml', 'https://feeds.bbci.co.uk/zhongwen/simp/rss.xml', 'rss', 'world', '["中文","新闻","world"]'::jsonb, 'zh', 'GB', true, 'official_seed', 'healthy'),
('nytimes-chinese', '纽约时报中文网', 'https://cn.nytimes.com/', 'https://cn.nytimes.com/rss/', 'https://cn.nytimes.com/rss/', 'rss', 'world', '["中文","新闻","analysis"]'::jsonb, 'zh', 'US', true, 'official_seed', 'healthy'),
('rfi-chinese', 'RFI 中文', 'https://www.rfi.fr/cn/', 'https://www.rfi.fr/cn/rss', 'https://www.rfi.fr/cn/rss', 'rss', 'world', '["中文","新闻","international"]'::jsonb, 'zh', 'FR', true, 'official_seed', 'healthy'),
('theinitium-rss', '端传媒', 'https://theinitium.com/', 'https://theinitium.com/rss', 'https://theinitium.com/rss', 'rss', 'world', '["中文","新闻","analysis"]'::jsonb, 'zh', 'HK', true, 'official_seed', 'healthy'),
('people-politics', '人民网时政', 'http://www.people.com.cn/rss/politics.xml', 'http://www.people.com.cn/rss/politics.xml', 'http://www.people.com.cn/rss/politics.xml', 'rss', 'world', '["中文","新闻","policy"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('xinhuanet-politics', '新华网时政', 'http://www.xinhuanet.com/politics/news_politics.xml', 'http://www.xinhuanet.com/politics/news_politics.xml', 'http://www.xinhuanet.com/politics/news_politics.xml', 'rss', 'world', '["中文","新闻","policy"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('solidot', 'Solidot', 'https://www.solidot.org/', 'https://www.solidot.org/index.rss', 'https://www.solidot.org/index.rss', 'rss', 'technology', '["中文","科技","open-source"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('sspai', '少数派', 'https://sspai.com/', 'https://sspai.com/feed', 'https://sspai.com/feed', 'rss', 'technology', '["中文","科技","product"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('ifanr', '爱范儿', 'https://www.ifanr.com/', 'https://www.ifanr.com/feed', 'https://www.ifanr.com/feed', 'rss', 'technology', '["中文","科技","consumer-tech"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('qbitai', '量子位', 'https://www.qbitai.com/', 'https://www.qbitai.com/feed', 'https://www.qbitai.com/feed', 'rss', 'ai', '["中文","AI","科技"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('infoq-cn', 'InfoQ 中文', 'https://www.infoq.cn/', 'https://www.infoq.cn/feed', 'https://www.infoq.cn/feed', 'rss', 'developer', '["中文","开发","架构"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('ruanyifeng-blog', '阮一峰的网络日志', 'https://www.ruanyifeng.com/blog/', 'https://www.ruanyifeng.com/blog/atom.xml', 'https://www.ruanyifeng.com/blog/atom.xml', 'atom', 'developer', '["中文","开发","web"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('cnblogs-sitehome', '博客园首页', 'https://www.cnblogs.com/', 'https://feed.cnblogs.com/blog/sitehome/rss', 'https://feed.cnblogs.com/blog/sitehome/rss', 'atom', 'developer', '["中文","开发","community"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('oschina-news', '开源中国资讯', 'https://www.oschina.net/news', 'https://www.oschina.net/news/rss', 'https://www.oschina.net/news/rss', 'rss', 'developer', '["中文","开源","开发"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('ithome', 'IT之家', 'https://www.ithome.com/', 'https://www.ithome.com/rss/', 'https://www.ithome.com/rss/', 'rss', 'technology', '["中文","科技","hardware"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('36kr', '36氪', 'https://36kr.com/', 'https://36kr.com/feed', 'https://36kr.com/feed', 'rss', 'business', '["中文","商业","创业"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('ftchinese', 'FT中文网', 'https://www.ftchinese.com/', 'http://www.ftchinese.com/rss/feed', 'http://www.ftchinese.com/rss/feed', 'rss', 'finance', '["中文","财经","analysis"]'::jsonb, 'zh', 'GB', true, 'official_seed', 'healthy'),
('douban-book-reviews', '豆瓣书评', 'https://book.douban.com/review/best/', 'https://www.douban.com/feed/review/book', 'https://www.douban.com/feed/review/book', 'rss', 'culture', '["中文","文化","books"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('apple-cn-newsroom', 'Apple 中国新闻稿', 'https://www.apple.com.cn/newsroom/', 'https://www.apple.com.cn/newsroom/rss-feed.rss', 'https://www.apple.com.cn/newsroom/rss-feed.rss', 'atom', 'technology', '["中文","科技","apple"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy'),
('microsoft-china-news', '微软中国新闻', 'https://news.microsoft.com/zh-cn/', 'https://news.microsoft.com/zh-cn/feed/', 'https://news.microsoft.com/zh-cn/feed/', 'rss', 'technology', '["中文","科技","microsoft"]'::jsonb, 'zh', 'CN', true, 'official_seed', 'healthy')
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
