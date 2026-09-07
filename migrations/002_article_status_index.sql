-- 状态统计使用覆盖索引，保留原有列表排序索引。
ALTER TABLE articles
    ADD INDEX idx_articles_author_status (author_id, deleted_at, status),
    ALGORITHM=INPLACE, LOCK=NONE;
