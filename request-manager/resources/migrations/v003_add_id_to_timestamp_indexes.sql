ALTER TABLE `requests`
  DROP INDEX `created_at`,
  ADD INDEX (`created_at`, `request_id`);
