-- Create acquirer table for storing acquirer information
CREATE TABLE IF NOT EXISTS acquirer (
    id BIGSERIAL PRIMARY KEY,
    ofe_identificacion VARCHAR(20) NOT NULL,
    adq_identificacion VARCHAR(20) NOT NULL,
    adq_tipo_adquirente VARCHAR(2),
    adq_id_personalizado VARCHAR(100),
    adq_informacion_personalizada JSONB,
    adq_razon_social VARCHAR(255) NOT NULL,
    adq_nombre_comercial VARCHAR(255),
    adq_primer_apellido VARCHAR(100),
    adq_segundo_apellido VARCHAR(100),
    adq_primer_nombre VARCHAR(100),
    adq_otros_nombres VARCHAR(100),
    tdo_codigo VARCHAR(10) NOT NULL,
    toj_codigo VARCHAR(10) NOT NULL,
    pai_codigo VARCHAR(10) NOT NULL,
    dep_codigo VARCHAR(10),
    dep_nombre VARCHAR(255),
    mun_codigo VARCHAR(10),
    mun_nombre VARCHAR(255),
    cpo_codigo VARCHAR(10),
    adq_direccion VARCHAR(255),
    adq_telefono VARCHAR(20),
    pai_codigo_domicilio_fiscal VARCHAR(10),
    dep_codigo_domicilio_fiscal VARCHAR(10),
    dep_nombre_domicilio_fiscal VARCHAR(255),
    mun_codigo_domicilio_fiscal VARCHAR(10),
    mun_nombre_domicilio_fiscal VARCHAR(255),
    cpo_codigo_domicilio_fiscal VARCHAR(10),
    adq_direccion_domicilio_fiscal VARCHAR(255),
    adq_nombre_contacto VARCHAR(255),
    adq_fax VARCHAR(20),
    adq_notas TEXT,
    adq_correo VARCHAR(255),
    adq_matricula_mercantil VARCHAR(50),
    adq_correos_notificacion TEXT,
    rfi_codigo VARCHAR(10),
    ref_codigo JSONB,
    responsable_tributos JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create unique index to ensure uniqueness of acquirer combination
-- This handles NULL values in adq_id_personalizado by treating them as empty strings
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_acquirer 
ON acquirer (ofe_identificacion, adq_identificacion, COALESCE(adq_id_personalizado, ''));

-- Create acquirer_contact table for storing contact information
CREATE TABLE IF NOT EXISTS acquirer_contact (
    id BIGSERIAL PRIMARY KEY,
    acquirer_id BIGINT NOT NULL REFERENCES acquirer(id) ON DELETE CASCADE,
    con_nombre VARCHAR(255) NOT NULL,
    con_direccion VARCHAR(255),
    con_telefono VARCHAR(20),
    con_correo VARCHAR(255),
    con_observaciones TEXT,
    con_tipo VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_acquirer_contact FOREIGN KEY (acquirer_id) REFERENCES acquirer(id) ON DELETE CASCADE
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_acquirer_ofe ON acquirer(ofe_identificacion);
CREATE INDEX IF NOT EXISTS idx_acquirer_adq ON acquirer(adq_identificacion);
CREATE INDEX IF NOT EXISTS idx_acquirer_ofe_adq ON acquirer(ofe_identificacion, adq_identificacion);
CREATE INDEX IF NOT EXISTS idx_acquirer_contact_acquirer ON acquirer_contact(acquirer_id);
CREATE INDEX IF NOT EXISTS idx_acquirer_created_at ON acquirer(created_at);

-- Add comments for documentation
COMMENT ON TABLE acquirer IS 'Stores acquirer information from OpenETL format';
COMMENT ON TABLE acquirer_contact IS 'Stores contact information associated with acquirers';
