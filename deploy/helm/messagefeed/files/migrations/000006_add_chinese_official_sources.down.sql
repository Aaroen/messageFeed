-- Helm 打包副本；源迁移位于项目 migrations 目录。
-- 移除本迁移新增的中文官方目录项；不影响用户已经导入到 sources 表的订阅。
DELETE FROM source_catalog_entries
WHERE source_origin = 'official_seed'
  AND source_key IN (
    'bbc-zhongwen',
    'nytimes-chinese',
    'rfi-chinese',
    'theinitium-rss',
    'people-politics',
    'xinhuanet-politics',
    'solidot',
    'sspai',
    'ifanr',
    'qbitai',
    'infoq-cn',
    'ruanyifeng-blog',
    'cnblogs-sitehome',
    'oschina-news',
    'ithome',
    '36kr',
    'ftchinese',
    'douban-book-reviews',
    'apple-cn-newsroom',
    'microsoft-china-news'
  );
