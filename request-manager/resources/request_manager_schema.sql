CREATE TABLE IF NOT EXISTS `requests` (
  `request_id`     BINARY(32) NOT NULL,
  `type`           VARBINARY(75) NOT NULL,
  `state`          TINYINT UNSIGNED NOT NULL DEFAULT 0,
  `user`           VARCHAR(100) NULL DEFAULT NULL,
  `created_at`     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `started_at`     TIMESTAMP NULL DEFAULT NULL,
  `finished_at`    TIMESTAMP NULL DEFAULT NULL,
  `total_jobs`     INT UNSIGNED NOT NULL DEFAULT 0,
  `finished_jobs`  INT UNSIGNED NOT NULL DEFAULT 0,

  PRIMARY KEY (`request_id`),
  INDEX (`created_at`),  -- recently created
  INDEX (`finished_at`), -- recently finished
  INDEX (`state`)        -- currently running
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `raw_requests` (
  `request_id` BINARY(32) NOT NULL,
  `request`    BLOB NOT NULL,
  `job_chain`  BLOB NOT NULL,

  PRIMARY KEY (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `job_log` (
  `request_id`  BINARY(32) NOT NULL,
  `job_id`      BINARY(4) NOT NULL,
  `try`         SMALLINT NOT NULL DEFAULT 0,
  `type`        VARBINARY(75) NOT NULL,
  `state`       TINYINT UNSIGNED NOT NULL DEFAULT 0,
  `started_at`  BIGINT UNSIGNED NOT NULL DEFAULT 0, -- Unix time (nanoseconds)
  `finished_at` BIGINT UNSIGNED NOT NULL DEFAULT 0, -- Unix time (nanoseconds)
  `error`       TEXT NULL DEFAULT NULL,
  `exit`        TINYINT UNSIGNED NULL DEFAULT NULL,
  `stdout`      LONGBLOB NULL DEFAULT NULL,
  `stderr`      LONGBLOB NULL DEFAULT NULL,

  PRIMARY KEY (`request_id`, `job_id`, `try`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
