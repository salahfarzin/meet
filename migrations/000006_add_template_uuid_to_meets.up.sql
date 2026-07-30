ALTER TABLE meets ADD COLUMN template_uuid VARCHAR(36) NULL AFTER settings, ADD KEY idx_template_uuid (template_uuid);
