package reception

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"3tcapital/ms_facturacion_core/internal/adapters/invoice/numrot"
)

// mapDocumentTypeIDToClasificacion mapea DocumentTypeId de Numrot a cdo_clasificacion
// Basado en tipos de documento DIAN:
// "01" -> "FC" (Factura Electrónica)
// "02" -> "FC" (Factura Electrónica de Exportación)
// "03" -> "NC" (Nota Crédito)
// "04" -> "ND" (Nota Débito)
// "05" -> "DS" (Documento Soporte)
func mapDocumentTypeIDToClasificacion(documentTypeID string) string {
	switch documentTypeID {
	case "01", "02":
		return "FC"
	case "03":
		return "NC"
	case "04":
		return "ND"
	case "05":
		return "DS"
	default:
		return "FC" // Default to FC for unknown types
	}
}

// parseEstadoFromMap intenta extraer información de estado desde el mapa Estado de Numrot
func parseEstadoFromMap(estadoMap map[string]string) (estado, resultado string) {
	if estadoMap == nil {
		return "", ""
	}

	// Intentar obtener estado y resultado del mapa
	estado = estadoMap["Estado"]
	if estado == "" {
		estado = estadoMap["estado"]
	}

	resultado = estadoMap["Resultado"]
	if resultado == "" {
		resultado = estadoMap["resultado"]
	}

	return estado, resultado
}

// mapEventCodeToEstado mapea códigos de evento de Numrot a nombres de estado legacy
// Basado en códigos de eventos DIAN:
// "030" -> "ACUSE" (Acuse de recibo)
// "031" -> "RECLAMO" (Reclamo)
// "032" -> "RECIBOBIEN" (Recibo del bien)
// "033" -> "ACEPTACION" (Aceptación expresa)
func mapEventCodeToEstado(codigo string) string {
	switch codigo {
	case "030":
		return "ACUSE"
	case "031":
		return "RECLAMO"
	case "032":
		return "RECIBOBIEN"
	case "033":
		return "ACEPTACION"
	default:
		return codigo // Return the code itself if unknown
	}
}

// parseEventosFromInterface intenta parsear el array de Eventos desde interface{}
// Retorna el último estado y el historial de estados
// La estructura de eventos de Numrot tiene:
// - Codigo: código del evento (e.g., "030", "032", "031")
// - Descripcion: descripción del evento
// - NumeroDocumento: objeto con FechaEmision y FechaFirma
// - ValidacionesDoc: array con objetos que tienen IsValida, Nombre, etc.
// urlPDF y urlXML son opcionales y se descargan y convierten a base64 para asignarlos a archivo y xml
func parseEventosFromInterface(ctx context.Context, eventos []interface{}, urlPDF, urlXML *string) (*UltimoEstado, []HistoricoEstado) {
	if len(eventos) == 0 {
		return nil, []HistoricoEstado{}
	}

	// Descargar y convertir URLs a base64 una sola vez para todos los estados
	var archivoBase64, xmlBase64 string
	if urlPDF != nil && *urlPDF != "" {
		archivoBase64 = downloadAndEncodeToBase64(ctx, *urlPDF)
	}
	if urlXML != nil && *urlXML != "" {
		xmlBase64 = downloadAndEncodeToBase64(ctx, *urlXML)
	}

	historico := make([]HistoricoEstado, 0, len(eventos))
	var ultimoEstado *UltimoEstado

	// Procesar eventos en orden cronológico
	for i, evento := range eventos {
		// Intentar convertir a map[string]interface{}
		eventoMap, ok := evento.(map[string]interface{})
		if !ok {
			continue
		}

		// Extraer campos del evento según la estructura de Numrot
		estado := ""
		resultado := ""
		mensajeResultado := ""
		fecha := ""

		// 1. Extraer Codigo y mapearlo a estado
		if codigo, ok := eventoMap["Codigo"].(string); ok {
			estado = mapEventCodeToEstado(codigo)
		}

		// 2. Extraer Descripcion como mensaje_resultado
		if descripcion, ok := eventoMap["Descripcion"].(string); ok {
			mensajeResultado = descripcion
		}

		// 3. Extraer fecha desde NumeroDocumento (nested structure)
		if numeroDoc, ok := eventoMap["NumeroDocumento"].(map[string]interface{}); ok {
			// Preferir FechaFirma, si no está disponible usar FechaEmision
			if fechaFirma, ok := numeroDoc["FechaFirma"].(string); ok && fechaFirma != "" {
				fecha = fechaFirma
			} else if fechaEmision, ok := numeroDoc["FechaEmision"].(string); ok && fechaEmision != "" {
				fecha = fechaEmision
			}
		}

		// 4. Extraer resultado desde ValidacionesDoc array
		// Si hay validaciones y todas son válidas, resultado = "EXITOSO"
		if validaciones, ok := eventoMap["ValidacionesDoc"].([]interface{}); ok {
			hasValidations := len(validaciones) > 0
			allValid := true
			for _, validacion := range validaciones {
				if validacionMap, ok := validacion.(map[string]interface{}); ok {
					if isValida, ok := validacionMap["IsValida"].(bool); ok {
						if !isValida {
							allValid = false
							break
						}
					}
				}
			}
			if hasValidations && allValid {
				resultado = "EXITOSO"
			} else if hasValidations {
				resultado = "FALLIDO"
			}
		}

		// Si no se pudo determinar resultado desde validaciones, usar default
		if resultado == "" {
			resultado = "EXITOSO" // Default si no hay validaciones
		}

		var mensajePtr *string
		if mensajeResultado != "" {
			mensajePtr = &mensajeResultado
		}

		// Usar los valores base64 descargados para archivo (PDF) y xml (XML)
		histEstado := HistoricoEstado{
			Estado:           estado,
			Resultado:        resultado,
			MensajeResultado: mensajePtr,
			Archivo:          archivoBase64,
			XML:              xmlBase64,
			Fecha:            fecha,
		}

		historico = append(historico, histEstado)

		// El último evento en el array es el último estado
		if i == len(eventos)-1 {
			ultimoEstado = &UltimoEstado{
				Estado:           estado,
				Resultado:        resultado,
				MensajeResultado: mensajeResultado,
				Archivo:          archivoBase64,
				XML:              xmlBase64,
				Fecha:            fecha,
			}
		}
	}

	return ultimoEstado, historico
}

// downloadAndEncodeToBase64 descarga el contenido de una URL y lo convierte a base64
// Retorna el string base64 o string vacío si hay error (no falla, solo deja vacío)
func downloadAndEncodeToBase64(ctx context.Context, url string) string {
	if url == "" {
		return ""
	}

	// Crear cliente HTTP con timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Crear request con contexto
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}

	// Ejecutar request
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Verificar status code
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Leer contenido
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	// Convertir a base64
	return base64.StdEncoding.EncodeToString(content)
}

// transformToLegacyFormat convierte datos de Numrot DocumentInfo a formato legacy
// urlPDF y urlXML son opcionales y se obtienen de GetDocumentByNumber cuando están disponibles
// Estas URLs se descargan y convierten a base64 para asignarlas a archivo y xml en los estados
func transformToLegacyFormat(ctx context.Context, docInfo *numrot.NumrotDocumentInfoData, urlPDF, urlXML *string) LegacyDocumentData {
	clasificacion := mapDocumentTypeIDToClasificacion(docInfo.DocumentTypeId)

	// Extraer estado y resultado del mapa Estado
	estado, resultado := parseEstadoFromMap(docInfo.Estado)
	if estado == "" {
		estado = "ACTIVO" // Default
	}

	// Descargar y convertir URLs a base64 para usar en estados
	var archivoBase64, xmlBase64 string
	if urlPDF != nil && *urlPDF != "" {
		archivoBase64 = downloadAndEncodeToBase64(ctx, *urlPDF)
	}
	if urlXML != nil && *urlXML != "" {
		xmlBase64 = downloadAndEncodeToBase64(ctx, *urlXML)
	}

	// Intentar parsear eventos para obtener ultimo_estado e historico_estados
	var ultimoEstado *UltimoEstado
	var historicoEstados []HistoricoEstado

	if len(docInfo.Eventos) > 0 {
		ultimoEstado, historicoEstados = parseEventosFromInterface(ctx, docInfo.Eventos, urlPDF, urlXML)
	}

	// Si no se pudo obtener ultimo_estado de eventos, intentar desde el mapa Estado
	if ultimoEstado == nil && estado != "" {
		ultimoEstado = &UltimoEstado{
			Estado:           estado,
			Resultado:        resultado,
			MensajeResultado: "",
			Archivo:          archivoBase64,
			XML:              xmlBase64,
			Fecha:            "",
		}
	}

	// Mapear campos adicionales si están disponibles
	var resolucionPtr *string
	if docInfo.Resolucion != "" {
		resolucionPtr = &docInfo.Resolucion
	}

	var horaDocumentoPtr *string
	if docInfo.HoraDocumento != "" {
		horaDocumentoPtr = &docInfo.HoraDocumento
	}

	var qrPtr *string
	if docInfo.QR != "" {
		qrPtr = &docInfo.QR
	}

	var signatureValuePtr *string
	if docInfo.SignatureValue != "" {
		signatureValuePtr = &docInfo.SignatureValue
	}

	return LegacyDocumentData{
		ID:                nil,
		OfeIdentificacion: docInfo.Receptor.NumeroDoc,
		ProIdentificacion: docInfo.Emisor.NumeroDoc,
		CdoClasificacion:  clasificacion,
		Resolucion:        resolucionPtr,
		Prefijo:           docInfo.NumeroDocumento.Serie,
		Consecutivo:       docInfo.NumeroDocumento.Folio,
		FechaDocumento:    docInfo.NumeroDocumento.FechaEmision,
		HoraDocumento:     horaDocumentoPtr,
		Estado:            estado,
		CUFE:              docInfo.UUID,
		QR:                qrPtr,
		SignatureValue:    signatureValuePtr,
		UltimoEstado:      ultimoEstado,
		HistoricoEstados:  historicoEstados,
	}
}
