-- Create provider table for storing provider information
CREATE TABLE IF NOT EXISTS provider (
    id BIGSERIAL PRIMARY KEY,
    ofe_identificacion VARCHAR(20) NOT NULL,
    pro_identificacion VARCHAR(20) NOT NULL,
    pro_id_personalizado VARCHAR(100),
    pro_razon_social VARCHAR(255),
    pro_nombre_comercial VARCHAR(255),
    pro_primer_apellido VARCHAR(100),
    pro_segundo_apellido VARCHAR(100),
    pro_primer_nombre VARCHAR(100),
    pro_otros_nombres VARCHAR(100),
    tdo_codigo VARCHAR(10) NOT NULL,
    toj_codigo VARCHAR(10) NOT NULL,
    pai_codigo VARCHAR(10),
    dep_codigo VARCHAR(10),
    mun_codigo VARCHAR(10),
    cpo_codigo VARCHAR(10),
    pro_direccion VARCHAR(255),
    pro_telefono VARCHAR(50) NOT NULL,
    pai_codigo_domicilio_fiscal VARCHAR(10),
    dep_codigo_domicilio_fiscal VARCHAR(10),
    mun_codigo_domicilio_fiscal VARCHAR(10),
    cpo_codigo_domicilio_fiscal VARCHAR(10),
    pro_direccion_domicilio_fiscal VARCHAR(255) NOT NULL,
    pro_correo VARCHAR(255) NOT NULL,
    pro_correos_notificacion TEXT,
    pro_matricula_mercantil VARCHAR(100),
    pro_usuarios_recepcion JSONB,
    rfi_codigo VARCHAR(10),
    ref_codigo JSONB,
    estado VARCHAR(20) DEFAULT 'ACTIVO' NOT NULL,
    fecha_creacion TIMESTAMP DEFAULT NOW(),
    fecha_modificacion TIMESTAMP DEFAULT NOW()
);

-- Create unique index to ensure uniqueness of provider combination
-- This handles NULL values in pro_id_personalizado by treating them as empty strings
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_provider 
ON provider (ofe_identificacion, pro_identificacion, COALESCE(pro_id_personalizado, ''));

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_provider_ofe ON provider(ofe_identificacion);
CREATE INDEX IF NOT EXISTS idx_provider_pro ON provider(pro_identificacion);
CREATE INDEX IF NOT EXISTS idx_provider_ofe_pro ON provider(ofe_identificacion, pro_identificacion);
CREATE INDEX IF NOT EXISTS idx_provider_estado ON provider(estado);
CREATE INDEX IF NOT EXISTS idx_provider_fecha_creacion ON provider(fecha_creacion);
CREATE INDEX IF NOT EXISTS idx_provider_razon_social ON provider(pro_razon_social);
CREATE INDEX IF NOT EXISTS idx_provider_nombre_comercial ON provider(pro_nombre_comercial);

-- Add comments for documentation
COMMENT ON TABLE provider IS 'Stores provider information from OpenETL format';
COMMENT ON COLUMN provider.estado IS 'Provider status: ACTIVO or INACTIVO';
COMMENT ON COLUMN provider.pro_usuarios_recepcion IS 'Array of user identifiers for reception';
COMMENT ON COLUMN provider.ref_codigo IS 'Array of fiscal responsibility codes';
