UPDATE items AS item
SET published_at = item.published_at - interval '8 hours', publication_verified = true
FROM sources AS source
WHERE source.id = item.source_id
  AND source.normalized_url IN ('https://www.infoq.cn/feed', 'https://infoq.cn/feed')
  AND item.published_at IS NOT NULL
  AND NOT item.publication_verified;

WITH evidence(source_url, article_path, published_at) AS (
    VALUES
        ('https://theinitium.com/rss', '/20261120-whatsnew-apec-shenzhen-explained', '2026-11-20 07:15:00+00'::timestamptz),
        ('https://feeds.bbci.co.uk/news/world/rss.xml', '/news/articles/c5yz9yj9x74o', '2026-06-28 11:44:23.548+00'::timestamptz),
        ('https://feeds.bbci.co.uk/news/world/rss.xml', '/news/articles/cj9gyk1z7ngo', '2026-06-28 10:11:35.353+00'::timestamptz),
        ('https://feeds.bbci.co.uk/news/world/rss.xml', '/news/articles/cr47dvywvy5o', '2026-06-28 12:39:41.026+00'::timestamptz),
        ('https://feeds.bbci.co.uk/zhongwen/simp/rss.xml', '/zhongwen/articles/cwyd8y1gy60o/trad', '2026-07-08 00:00:59.557+00'::timestamptz),
        ('https://feeds.bbci.co.uk/zhongwen/simp/rss.xml', '/zhongwen/articles/cvgjm0x0pzlo/trad', '2026-07-09 04:39:15.129+00'::timestamptz),
        ('https://feeds.bbci.co.uk/zhongwen/simp/rss.xml', '/zhongwen/articles/c802k9v735no/trad', '2026-06-19 11:25:54.229+00'::timestamptz),
        ('https://sspai.com/feed', '/post/111709', '2026-07-01 09:30:00+00'::timestamptz)
)
UPDATE items AS item
SET published_at = evidence.published_at, publication_verified = true
FROM sources AS source, evidence
WHERE source.id = item.source_id
  AND rtrim(source.normalized_url, '/') = evidence.source_url
  AND rtrim(regexp_replace(split_part(item.normalized_url, '?', 1), '^https?://[^/]+', ''), '/') = evidence.article_path
  AND (item.published_at IS DISTINCT FROM evidence.published_at OR NOT item.publication_verified);
