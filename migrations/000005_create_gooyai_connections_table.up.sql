-- A clinic's connection to Gooyai, the AI phone assistant (dashboard
-- Settings -> Booking Settings -> Phone assistant). One row per organizer.
--
-- api_key_ciphertext is the clinic's Gooyai Partner API key sealed with
-- AES-256-GCM under GOOYAI_ENCRYPTION_KEY (nonce prepended); the plaintext is
-- never stored or returned. key_last4 is kept in the clear only so the
-- dashboard can show which key is connected ("…ab12").
--
-- environment picks Gooyai's sandbox or production API.
--
-- Only credentials live here. Sync state (Gooyai booking ids, which slots we
-- own) stays in meets.settings and in Gooyai itself.
CREATE TABLE gooyai_connections (
    organizer_uuid VARCHAR(36) NOT NULL PRIMARY KEY,
    api_key_ciphertext VARBINARY(512) NOT NULL,
    key_last4 VARCHAR(4) NOT NULL,
    environment ENUM('sandbox','prod') NOT NULL DEFAULT 'sandbox',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
