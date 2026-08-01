ALTER TABLE meets
    DROP PRIMARY KEY,
    DROP INDEX unique_uuid,
    DROP COLUMN id,
    ADD PRIMARY KEY (uuid);
