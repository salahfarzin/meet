CREATE TABLE availability_template_occurrences (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    template_uuid VARCHAR(36) NOT NULL,
    occurrence_date DATE NOT NULL,
    status ENUM('materialized','skipped') NOT NULL,
    meet_uuid VARCHAR(36) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_template_date (template_uuid, occurrence_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
