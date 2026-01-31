# Microservicio de Clonación

Microservicio en Go para gestionar el módulo de clonación de trámites.

## Estructura del Proyecto

```
clonacion-service/
├── internal/
│   ├── models/          # Modelos de dominio
│   ├── dto/             # Data Transfer Objects (requests/responses)
│   ├── repository/      # Interfaces de repositorio
│   │   └── postgres/    # Implementación PostgreSQL
│   ├── service/         # Lógica de negocio
│   ├── handler/         # Handlers HTTP
│   └── router/          # Configuración de rutas
├── migrations/          # Scripts SQL de migración
├── main.go              # Punto de entrada
└── go.mod               # Dependencias
```

## Características

- ✅ Arquitectura limpia y modular
- ✅ Separación de responsabilidades (Repository, Service, Handler)
- ✅ Sistema de estados y transiciones
- ✅ Optimistic locking con versionado
- ✅ Soft delete
- ✅ Paginación
- ✅ Validación de transiciones de estado

## Endpoints

### Clonaciones

- `POST /clonaciones` - Crear una nueva clonación
- `GET /clonaciones/{id}` - Obtener clonación por ID
- `GET /clonaciones?page=1&size=10` - Listar clonaciones (paginado)
- `PUT /clonaciones/{id}/responder` - Responder una clonación
- `POST /clonaciones/{id}/asignar` - Asignar una clonación
- `POST /clonaciones/{id}/rechazar` - Rechazar una clonación
- `GET /clonaciones/{id}/adjuntos` - Obtener adjuntos de una clonación

### Health Check

- `GET /health` - Verificar estado del servicio

## Estados y Transiciones

### Estados Disponibles

- `CLONACION_CREADA` - Estado inicial
- `CLONACION_ASIGNADA` - Asignada a un usuario
- `CLONACION_EN_EDICION` - En proceso de edición
- `CLONACION_RESPONDIDA` - Respondida exitosamente
- `CLONACION_RECHAZADA` - Rechazada

### Transiciones Permitidas

- `CLONACION_CREADA` → `CLONACION_ASIGNADA`
- `CLONACION_ASIGNADA` → `CLONACION_EN_EDICION` | `CLONACION_RECHAZADA`
- `CLONACION_EN_EDICION` → `CLONACION_RESPONDIDA` | `CLONACION_RECHAZADA`

## Instalación

1. Instalar dependencias:
```bash
go mod download
```

2. Configurar base de datos PostgreSQL y ejecutar migraciones:
```bash
psql -U usuario -d base_de_datos -f migrations/001_create_tables.sql
```

3. Configurar variables de entorno:
```bash
export PORT=8080
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=usuario
export DB_PASSWORD=password
export DB_NAME=clonacion_db
```

4. Ejecutar el servicio:
```bash
go run main.go
```

## Próximos Pasos

1. **Implementar repositorios completos**: Completar las implementaciones de `AdjuntoRepository` y `RespuestaRepository` en `internal/repository/postgres/`

2. **Configuración de base de datos**: Agregar conexión a PostgreSQL en `main.go`

3. **Validación**: Integrar una librería de validación como `go-playground/validator`

4. **Middleware**: Agregar middleware para:
   - Autenticación/autorización
   - Logging
   - CORS
   - Rate limiting

5. **Testing**: Agregar tests unitarios e integración

6. **Documentación API**: Integrar Swagger/OpenAPI

7. **Manejo de adjuntos**: Implementar subida y almacenamiento de archivos (S3, local, etc.)

## Ejemplo de Uso

### Crear una clonación

```bash
curl -X POST http://localhost:8080/clonaciones \
  -H "Content-Type: application/json" \
  -d '{
    "tramiteId": "123e4567-e89b-12d3-a456-426614174000",
    "usuarioClonadoId": "123e4567-e89b-12d3-a456-426614174001",
    "usuarioAsignadorId": "123e4567-e89b-12d3-a456-426614174002",
    "motivo": "Clonación por solicitud del usuario",
    "tiempoAsignado": {
      "valor": 24,
      "unidad": "HOURS"
    }
  }'
```

### Responder una clonación

```bash
curl -X PUT http://localhost:8080/clonaciones/{id}/responder \
  -H "Content-Type: application/json" \
  -d '{
    "usuarioRespuestaId": "123e4567-e89b-12d3-a456-426614174001",
    "parrafo": "Respuesta a la clonación solicitada",
    "adjuntos": []
  }'
```

## Notas

- El servicio está diseñado para ser reutilizable y extensible
- Los repositorios usan interfaces para facilitar testing y cambios de implementación
- El sistema de estados y transiciones es validado en el servicio
- Se implementa optimistic locking para prevenir condiciones de carrera
