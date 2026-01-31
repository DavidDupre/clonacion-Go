-- Make pro_telefono nullable in provider table
ALTER TABLE provider ALTER COLUMN pro_telefono DROP NOT NULL;
