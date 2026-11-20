UPDATE items AS item
SET published_at = '2026-07-10 05:33:28+00'::timestamptz, publication_verified = true
FROM sources AS source
WHERE source.id = item.source_id
  AND rtrim(source.normalized_url, '/') = 'https://www.qbitai.com/feed'
  AND split_part(item.normalized_url, '?', 1) = 'https://www.qbitai.com/2026/07/436498.html'
  AND item.published_at = '2026-07-18 05:33:28+00'::timestamptz;
