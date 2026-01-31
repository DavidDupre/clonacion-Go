-- Alter tramite_id from UUID to VARCHAR
ALTER TABLE clonaciones
ALTER COLUMN tramite_id TYPE VARCHAR(255);

