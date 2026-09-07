ALTER TABLE meets
    ADD COLUMN settings JSON NULL AFTER booked_at,
    ADD COLUMN creator_uuid VARCHAR(36) NULL AFTER organizer_uuid,
    ADD COLUMN version INT NOT NULL DEFAULT 1 AFTER settings,
    ADD COLUMN template_uuid VARCHAR(36) NULL AFTER settings,
    ADD KEY idx_template_uuid (template_uuid);

-- uuid is v7 (time-ordered, generated in Go via uuid.NewV7()), so using it
-- directly as PRIMARY KEY doesn't fragment InnoDB's clustered index the way
-- a random v4 uuid would - no separate BIGINT surrogate id needed.
CREATE TABLE availability_templates (
    uuid VARCHAR(36) NOT NULL PRIMARY KEY,
    organizer_uuid VARCHAR(36) NOT NULL,
    weekday TINYINT NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    price_uuid VARCHAR(36) NULL,
    effective_from DATE NOT NULL,
    effective_until DATE NULL,
    active TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_organizer_uuid (organizer_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE availability_template_occurrences (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    template_uuid VARCHAR(36) NOT NULL,
    occurrence_date DATE NOT NULL,
    status ENUM('materialized','skipped') NOT NULL,
    meet_uuid VARCHAR(36) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_template_date (template_uuid, occurrence_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE meets
    DROP PRIMARY KEY,
    DROP INDEX unique_uuid,
    DROP COLUMN id,
    ADD PRIMARY KEY (uuid);

ALTER TABLE meets
    ADD INDEX idx_organizer_created_uuid (organizer_uuid, created_at, uuid),
    ADD INDEX idx_organizer_start_uuid   (organizer_uuid, start_time, uuid),
    ADD INDEX idx_organizer_end_uuid     (organizer_uuid, end_time, uuid);
