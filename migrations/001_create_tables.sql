-- Migración para crear las tablas del módulo de Clonación

-- Tabla de clonaciones
CREATE TABLE IF NOT EXISTS clonaciones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tramite_id UUID NOT NULL,
    usuario_clonado_id UUID NOT NULL,
    usuario_asignador_id UUID NOT NULL,
    motivo TEXT NOT NULL,
    estado VARCHAR(50) NOT NULL DEFAULT 'CLONACION_CREADA',
    tiempo_asignado_valor INTEGER NOT NULL,
    tiempo_asignado_unidad VARCHAR(20) NOT NULL CHECK (tiempo_asignado_unidad IN ('MINUTES', 'HOURS', 'DAYS')),
    fecha_creacion TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fecha_vencimiento TIMESTAMPTZ,
    fecha_respuesta TIMESTAMPTZ,
    contador_rechazos INTEGER DEFAULT 0,
    usuario_respuesta_id UUID,
    metadata JSONB,
    version INTEGER DEFAULT 1,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL
);

-- Índices para clonaciones
CREATE INDEX IF NOT EXISTS idx_clonaciones_tramite_id ON clonaciones(tramite_id);
CREATE INDEX IF NOT EXISTS idx_clonaciones_estado ON clonaciones(estado);
CREATE INDEX IF NOT EXISTS idx_clonaciones_usuario_clonado_id ON clonaciones(usuario_clonado_id);
CREATE INDEX IF NOT EXISTS idx_clonaciones_deleted_at ON clonaciones(deleted_at) WHERE deleted_at IS NULL;

-- Tabla de adjuntos
CREATE TABLE IF NOT EXISTS clonacion_adjuntos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clonacion_id UUID NOT NULL REFERENCES clonaciones(id) ON DELETE CASCADE,
    nombre VARCHAR(255) NOT NULL,
    ruta_url TEXT NOT NULL,
    tamaño BIGINT NOT NULL,
    tipo VARCHAR(100) NOT NULL,
    creado_por UUID NOT NULL,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índices para adjuntos
CREATE INDEX IF NOT EXISTS idx_clonacion_adjuntos_clonacion_id ON clonacion_adjuntos(clonacion_id);

-- Tabla de respuestas
CREATE TABLE IF NOT EXISTS clonacion_respuestas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clonacion_id UUID NOT NULL REFERENCES clonaciones(id) ON DELETE CASCADE,
    parrafo TEXT NOT NULL,
    usuario_respuesta_id UUID NOT NULL,
    fecha_respuesta TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    estado_resultado VARCHAR(50) NOT NULL,
    metadata JSONB
);

-- Índices para respuestas
CREATE INDEX IF NOT EXISTS idx_clonacion_respuestas_clonacion_id ON clonacion_respuestas(clonacion_id);
