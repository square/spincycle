ALTER TABLE `requests`
  DROP INDEX `state`,
  ADD INDEX (`state`, `created_at`)