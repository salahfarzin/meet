-- Admin-managed catalogue a trappist picks from for their public profile
-- (dashboard Settings -> Appointments). kind separates clinical specialties
-- ("anxiety", "couples") from therapy approaches ("cbt", "emdr") so both
-- lists share one table and one admin screen.
--
-- Trappists store the picked uuids in psychometrist's
-- users.center_details.appointments.profile; this table only owns the
-- catalogue and its translations. names is a {"en": "...", "fa": "..."} map
-- so every client resolves the label in its own UI language.
CREATE TABLE specialties (
    uuid VARCHAR(36) NOT NULL PRIMARY KEY,
    kind ENUM('specialty','approach') NOT NULL,
    slug VARCHAR(64) NOT NULL,
    names JSON NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    active TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_kind_slug (kind, slug),
    KEY idx_kind_sort (kind, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
