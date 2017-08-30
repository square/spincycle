CREATE TABLE IF NOT EXISTS `requests` (
  `id`             BINARY(32) NOT NULL,
  `type`           VARBINARY(75) NOT NULL,
  `state`          TINYINT NOT NULL DEFAULT 0,
  `user`           VARBINARY(32) NOT NULL DEFAULT "?",
  `created_at`     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `started_at`     TIMESTAMP NULL DEFAULT NULL,
  `finished_at`    TIMESTAMP NULL DEFAULT NULL,
  `total_jobs`     INT DEFAULt 0,
  `finished_jobs`  INT DEFAULT 0,

  PRIMARY KEY (`id`),
  INDEX created_at (`created_at`),
  INDEX state (`state`)
);

CREATE TABLE IF NOT EXISTS `raw_requests` (
  `request_id` BINARY(32) NOT NULL,
  `request`    BLOB NOT NULL,
  `job_chain`  BLOB NOT NULL,

  PRIMARY KEY (`request_id`)
);

CREATE TABLE IF NOT EXISTS `job_log` (
  `request_id`  BINARY(32) NOT NULL,
  `job_id`      VARBINARY(100) NOT NULL,
  `try`         SMALLINT NOT NULL DEFAULT 0,
  `type`        VARBINARY(75) NOT NULL,
  `state`       TINYINT NOT NULL DEFAULT 0,
  `status`      TEXT NULL DEFAULT NULL,
  `started_at`  TIMESTAMP NULL DEFAULT NULL,
  `finished_at` TIMESTAMP NULL DEFAULT NULL,
  `error`       TEXT NULL DEFAULT NULL,
  `exit`        TINYINT UNSIGNED NULL DEFAULT NULL,
  `stdout`      TEXT NULL DEFAULT NULL,
  `stderr`      TEXT NULL DEFAULT NULL,

  PRIMARY KEY (`request_id`, `job_id`, `try`)
);
