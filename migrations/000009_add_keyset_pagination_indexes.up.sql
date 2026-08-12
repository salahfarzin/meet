ALTER TABLE meets
    ADD INDEX idx_organizer_created_uuid (organizer_uuid, created_at, uuid),
    ADD INDEX idx_organizer_start_uuid   (organizer_uuid, start_time, uuid),
    ADD INDEX idx_organizer_end_uuid     (organizer_uuid, end_time, uuid);
