# Implementación del Microservicio de Clonación

## ✅ Componentes Implementados

### 1. Modelos de Dominio (`internal/models/`)
- ✅ **Clonacion**: Modelo principal con todos los campos requeridos
- ✅ **DocumentoAdjunto**: Modelo para archivos adjuntos
- ✅ **RespuestaClonacion**: Modelo para respuestas
- ✅ **Sistema de Estados y Transiciones**: Validación de transiciones de estado

### 2. DTOs (`internal/dto/`)
- ✅ **Requests**: CreateClonacionRequest, ResponderClonacionRequest, etc.
- ✅ **Responses**: ClonacionResponse, PaginatedResponse, ErrorResponse, etc.

### 3. Repositorios (`internal/repository/`)
- ✅ **Interfaces**: Definidas para todos los repositorios
- ✅ **Implementación PostgreSQL**: 
  - ClonacionRepository (completo)
  - AdjuntoRepository (completo)
  - RespuestaRepository (completo)

### 4. Servicios (`internal/service/`)
- ✅ **ClonacionService**: Lógica de negocio completa
  - Crear clonación
  - Obtener por ID
  - Listar con paginación
  - Responder clonación
  - Asignar clonación
  - Rechazar clonación
  - Obtener adjuntos

### 5. Handlers HTTP (`internal/handler/`)
- ✅ Todos los endpoints implementados:
  - POST /clonaciones
  - GET /clonaciones/{id}
  - GET /clonaciones?page=&size=
  - PUT /clonaciones/{id}/responder
  - POST /clonaciones/{id}/asignar
  - POST /clonaciones/{id}/rechazar
  - GET /clonaciones/{id}/adjuntos

### 6. Router (`internal/router/`)
- ✅ Configuración completa de rutas
- ✅ Health check endpoint

### 7. Base de Datos
- ✅ Scripts de migración SQL (`migrations/001_create_tables.sql`)
- ✅ Índices optimizados
- ✅ Soft delete implementado
- ✅ Optimistic locking con versionado

## 🏗️ Arquitectura

El proyecto sigue una arquitectura limpia y modular:

```
┌─────────────┐
│   Handler   │  ← Capa HTTP (REST API)
└──────┬──────┘
       │
┌──────▼──────┐
│   Service   │  ← Lógica de Negocio
└──────┬──────┘
       │
┌──────▼──────┐
│ Repository  │  ← Acceso a Datos
└──────┬──────┘
       │
┌──────▼──────┐
│  Database   │  ← PostgreSQL
└─────────────┘
```

## 🔧 Características Implementadas

### Validaciones
- ✅ Campos requeridos en creación
- ✅ Validación de transiciones de estado
- ✅ Optimistic locking para prevenir condiciones de carrera

### Estados y Transiciones
- ✅ Sistema completo de estados
- ✅ Validación de transiciones permitidas
- ✅ Retorno de `allowedTransitions` en respuestas

### Paginación
- ✅ Implementada en listado de clonaciones
- ✅ Parámetros: page, size
- ✅ Respuesta incluye total y totalPages

## 📝 Próximos Pasos para Completar

1. **Instalar dependencias**:
   ```bash
   go mod download
   ```

2. **Configurar base de datos**:
   - Crear base de datos PostgreSQL
   - Ejecutar migraciones: `psql -U usuario -d base_de_datos -f migrations/001_create_tables.sql`

3. **Configurar variables de entorno**:
   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=postgres
   export DB_PASSWORD=postgres
   export DB_NAME=clonacion_db
   export PORT=8080
   ```

4. **Actualizar main.go**:
   - Copiar el contenido de `example_main.go` a `main.go`
   - O seguir las instrucciones comentadas en `main.go`

5. **Agregar middleware** (opcional pero recomendado):
   - Autenticación/autorización
   - Logging
   - CORS
   - Rate limiting

6. **Agregar validación de requests**:
   - Integrar `go-playground/validator` o similar

7. **Implementar manejo de adjuntos**:
   - Subida de archivos
   - Almacenamiento (S3, local, etc.)
   - Generación de presigned URLs

## 🚀 Uso del Servicio

Una vez configurado, el servicio estará disponible en `http://localhost:8080`

### Ejemplo: Crear Clonación
```bash
curl -X POST http://localhost:8080/clonaciones \
  -H "Content-Type: application/json" \
  -d '{
    "tramiteId": "123e4567-e89b-12d3-a456-426614174000",
    "usuarioClonadoId": "123e4567-e89b-12d3-a456-426614174001",
    "usuarioAsignadorId": "123e4567-e89b-12d3-a456-426614174002",
    "motivo": "Clonación por solicitud",
    "tiempoAsignado": {
      "valor": 24,
      "unidad": "HOURS"
    }
  }'
```

### Ejemplo: Responder Clonación
```bash
curl -X PUT http://localhost:8080/clonaciones/{id}/responder \
  -H "Content-Type: application/json" \
  -d '{
    "usuarioRespuestaId": "123e4567-e89b-12d3-a456-426614174001",
    "parrafo": "Respuesta a la clonación",
    "adjuntos": []
  }'
```

## 📦 Estructura de Archivos

```
clonacion-service/
├── internal/
│   ├── models/
│   │   ├── clonacion.go          # Modelos de dominio
│   │   └── transiciones.go       # Sistema de estados
│   ├── dto/
│   │   ├── requests.go           # DTOs de entrada
│   │   └── responses.go          # DTOs de salida
│   ├── repository/
│   │   ├── clonacion_repository.go  # Interfaces
│   │   └── postgres/
│   │       ├── clonacion_repository.go
│   │       ├── adjunto_repository.go
│   │       └── respuesta_repository.go
│   ├── service/
│   │   └── clonacion_service.go  # Lógica de negocio
│   ├── handler/
│   │   └── clonacion_handler.go  # Handlers HTTP
│   └── router/
│       └── router.go              # Configuración de rutas
├── migrations/
│   └── 001_create_tables.sql    # Scripts SQL
├── main.go                        # Punto de entrada
├── example_main.go                # Ejemplo de inicialización
├── go.mod                         # Dependencias
└── README.md                      # Documentación
```

## ✨ Características de Reutilización

El código está diseñado para ser reutilizable:

1. **Interfaces**: Todos los repositorios usan interfaces, facilitando cambios de implementación
2. **Separación de responsabilidades**: Cada capa tiene una responsabilidad clara
3. **Inyección de dependencias**: Los servicios reciben sus dependencias como parámetros
4. **Extensible**: Fácil agregar nuevos endpoints o funcionalidades
5. **Testeable**: La arquitectura facilita la escritura de tests unitarios

## 🔒 Seguridad y Buenas Prácticas

- ✅ Soft delete implementado
- ✅ Optimistic locking para prevenir condiciones de carrera
- ✅ Validación de transiciones de estado
- ✅ Manejo de errores estructurado
- ✅ Códigos HTTP estándar (200, 201, 400, 404, 500)
- ⚠️ Pendiente: Autenticación/autorización (agregar middleware)
- ⚠️ Pendiente: Validación de inputs (agregar librería de validación)
