-- Make pro_direccion_domicilio_fiscal nullable in provider table
ALTER TABLE provider ALTER COLUMN pro_direccion_domicilio_fiscal DROP NOT NULL;

