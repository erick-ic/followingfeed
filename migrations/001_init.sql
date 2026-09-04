-- FollowingFeed 首次生产发布的完整数据库基线。
-- 后续结构变更必须新增迁移文件，禁止修改已经执行过的基线。

CREATE TABLE users (
  id BIGINT NOT NULL AUTO_INCREMENT,
  nickname VARCHAR(24) NOT NULL,
  email VARCHAR(255) NOT NULL,
  password VARCHAR(255) NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE articles (
  id BIGINT NOT NULL AUTO_INCREMENT,
  title VARCHAR(1024) NOT NULL,
  content BLOB NOT NULL,
  author_id BIGINT NOT NULL,
  status TINYINT UNSIGNED NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  deleted_at BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_articles_author_list (author_id, deleted_at, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE publish_articles (
  id BIGINT NOT NULL,
  title VARCHAR(1024) NOT NULL,
  content BLOB NOT NULL,
  author_id BIGINT NOT NULL,
  status TINYINT UNSIGNED NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  deleted_at BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_publish_articles_public_list (status, deleted_at, updated_at, id),
  KEY idx_publish_articles_author_list (author_id, status, deleted_at, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE follows (
  id BIGINT NOT NULL AUTO_INCREMENT,
  follower_id BIGINT NOT NULL,
  following_id BIGINT NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_follows_follower_following (follower_id, following_id),
  KEY idx_follows_follower_created (follower_id, created_at, id),
  KEY idx_follows_following_created (following_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE interactives (
  id BIGINT NOT NULL AUTO_INCREMENT,
  biz_id BIGINT NOT NULL,
  biz VARCHAR(64) NOT NULL,
  read_cnt BIGINT NOT NULL DEFAULT 0,
  like_cnt BIGINT NOT NULL DEFAULT 0,
  collect_cnt BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_interactives_biz (biz_id, biz)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_like_bizs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  uid BIGINT NOT NULL,
  biz_id BIGINT NOT NULL,
  biz VARCHAR(64) NOT NULL,
  status TINYINT UNSIGNED NOT NULL DEFAULT 1,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_like_biz (uid, biz_id, biz),
  KEY idx_user_like_biz_target (biz, biz_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_collection_bizs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  uid BIGINT NOT NULL,
  biz_id BIGINT NOT NULL,
  biz VARCHAR(64) NOT NULL,
  status TINYINT UNSIGNED NOT NULL DEFAULT 1,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_collection_biz (uid, biz_id, biz),
  KEY idx_user_collection_target (biz, biz_id, status),
  KEY idx_user_collection_list (uid, biz, status, updated_at, id, biz_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
