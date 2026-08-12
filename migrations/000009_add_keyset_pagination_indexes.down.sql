ALTER TABLE meets
    DROP INDEX idx_organizer_created_uuid,
    DROP INDEX idx_organizer_start_uuid,
    DROP INDEX idx_organizer_end_uuid;
