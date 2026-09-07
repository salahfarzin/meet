ALTER TABLE meets
    DROP INDEX idx_organizer_created_uuid,
    DROP INDEX idx_organizer_start_uuid,
    DROP INDEX idx_organizer_end_uuid;

ALTER TABLE meets
    DROP PRIMARY KEY,
    ADD COLUMN id BIGINT AUTO_INCREMENT FIRST,
    ADD PRIMARY KEY (id),
    ADD UNIQUE KEY unique_uuid (uuid);

DROP TABLE IF EXISTS availability_template_occurrences;

DROP TABLE IF EXISTS availability_templates;

ALTER TABLE meets
    DROP KEY idx_template_uuid,
    DROP COLUMN template_uuid,
    DROP COLUMN version,
    DROP COLUMN creator_uuid,
    DROP COLUMN settings;
