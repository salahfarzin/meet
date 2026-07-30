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
