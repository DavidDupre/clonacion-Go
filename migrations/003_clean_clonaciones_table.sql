-- Script para recrear la tabla clonaciones solo con campos necesarios

-- Eliminar las tablas dependientes primero
DROP TABLE IF EXISTS clonacion_respuestas CASCADE;
DROP TABLE IF EXISTS clonacion_adjuntos CASCADE;
DROP TABLE IF EXISTS clonaciones CASCADE;

-- Crear tabla limpia con solo los campos necesarios
CREATE TABLE clonaciones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tramite_id VARCHAR(255) NOT NULL,
    usuario_clonado_id UUID NOT NULL,
    usuario_asignador_id UUID NOT NULL,
    motivo TEXT NOT NULL,
    estado VARCHAR(50) NOT NULL DEFAULT 'CLONACION_CREADA',
    contador_rechazos INTEGER DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Crear índices para búsquedas frecuentes
CREATE INDEX idx_clonaciones_tramite_id ON clonaciones(tramite_id);
CREATE INDEX idx_clonaciones_estado ON clonaciones(estado);
CREATE INDEX idx_clonaciones_usuario_clonado_id ON clonaciones(usuario_clonado_id);
CREATE INDEX idx_clonaciones_deleted_at ON clonaciones(deleted_at) WHERE deleted_at IS NULL;

-- Tabla de adjuntos (simplificada)
CREATE TABLE clonacion_adjuntos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clonacion_id UUID NOT NULL REFERENCES clonaciones(id) ON DELETE CASCADE,
    nombre VARCHAR(255) NOT NULL,
    ruta_url TEXT NOT NULL,
    tipo VARCHAR(100) NOT NULL,
    tamaño BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clonacion_adjuntos_clonacion_id ON clonacion_adjuntos(clonacion_id);

-- Tabla de respuestas (simplificada)
CREATE TABLE clonacion_respuestas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clonacion_id UUID NOT NULL REFERENCES clonaciones(id) ON DELETE CASCADE,
    usuario_respuesta_id UUID NOT NULL,
    parrafo TEXT NOT NULL,
    estado_resultado VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clonacion_respuestas_clonacion_id ON clonacion_respuestas(clonacion_id);
