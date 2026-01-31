package security

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Sensitive header names that should be redacted.
var sensitiveHeaders = map[string]bool{
	"authorization":  true,
	"cookie":         true,
	"set-cookie":     true,
	"x-api-key":      true,
	"x-auth-token":   true,
	"proxy-authorization": true,
}

// Sensitive field names in JSON bodies that should be redacted.
var sensitiveFields = []string{
	"password",
	"secret",
	"token",
	"key",
	"authorization",
	"api_key",
	"apikey",
	"access_token",
	"refresh_token",
	"client_secret",
	"private_key",
	"credential",
	"auth",
}

const redactedValue = "[REDACTED]"

// SanitizeHeaders removes sensitive headers from an HTTP header map.
// Returns a new map with sensitive values redacted.
func SanitizeHeaders(headers http.Header) map[string]string {
	sanitized := make(map[string]string)
	
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = redactedValue
		} else {
			// Join multiple values with comma
			sanitized[key] = strings.Join(values, ", ")
		}
	}
	
	return sanitized
}

// SanitizeBody removes sensitive fields from a JSON body.
// Returns sanitized JSON bytes. Handles gzip-compressed and binary data properly.
func SanitizeBody(body []byte, maxSize int) json.RawMessage {
	if len(body) == 0 {
		return nil
	}

	// Check if body is gzip-compressed (magic number: 0x1f 0x8b)
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		decompressed, err := decompressGzip(body)
		if err == nil {
			// Successfully decompressed, use the decompressed data
			body = decompressed
		} else {
			// Could not decompress, store as base64 encoded
			return wrapBinaryAsJSON(body, "gzip-compressed (decompression failed)")
		}
	}

	// Check if body contains valid UTF-8
	if !utf8.Valid(body) {
		// Binary data that's not valid UTF-8, encode as base64
		return wrapBinaryAsJSON(body, "binary (non-UTF8)")
	}

	// Truncate if too large
	if maxSize > 0 && len(body) > maxSize {
		truncated := map[string]interface{}{
			"_truncated": true,
			"_size":      len(body),
			"_preview":   string(body[:maxSize]),
		}
		result, _ := json.Marshal(truncated)
		return json.RawMessage(result)
	}

	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Valid UTF-8 but not JSON, wrap as string in JSON object
		wrapped := map[string]interface{}{
			"_raw":    string(body),
			"_format": "text",
		}
		result, _ := json.Marshal(wrapped)
		return json.RawMessage(result)
	}

	// Recursively sanitize the data
	sanitized := sanitizeValue(data)

	// Marshal back to JSON
	result, err := json.Marshal(sanitized)
	if err != nil {
		// Fallback to wrapped string if marshaling fails
		wrapped := map[string]interface{}{
			"_raw":    string(body),
			"_format": "text",
		}
		result, _ = json.Marshal(wrapped)
		return json.RawMessage(result)
	}

	return json.RawMessage(result)
}

// decompressGzip attempts to decompress gzip-compressed data.
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}

// wrapBinaryAsJSON wraps binary data as a JSON object with base64 encoding.
func wrapBinaryAsJSON(data []byte, format string) json.RawMessage {
	wrapped := map[string]interface{}{
		"_binary":  true,
		"_format":  format,
		"_size":    len(data),
		"_base64":  base64.StdEncoding.EncodeToString(data),
	}
	result, _ := json.Marshal(wrapped)
	return json.RawMessage(result)
}

// sanitizeValue recursively sanitizes a JSON value.
func sanitizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return sanitizeMap(val)
	case []interface{}:
		return sanitizeSlice(val)
	default:
		return val
	}
}

// sanitizeMap sanitizes a JSON object by redacting sensitive fields.
func sanitizeMap(m map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	
	for key, value := range m {
		lowerKey := strings.ToLower(key)
		
		// Check if this is a sensitive field
		isSensitive := false
		for _, sensitiveField := range sensitiveFields {
			if strings.Contains(lowerKey, sensitiveField) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			sanitized[key] = redactedValue
		} else {
			sanitized[key] = sanitizeValue(value)
		}
	}
	
	return sanitized
}

// sanitizeSlice sanitizes a JSON array by recursively sanitizing each element.
func sanitizeSlice(s []interface{}) []interface{} {
	sanitized := make([]interface{}, len(s))
	
	for i, value := range s {
		sanitized[i] = sanitizeValue(value)
	}
	
	return sanitized
}

// SanitizeURL redacts sensitive query parameters from a URL.
func SanitizeURL(url string) string {
	// Simple approach: if URL contains sensitive parameter names, mark as sensitive
	lowerURL := strings.ToLower(url)
	
	for _, sensitiveField := range sensitiveFields {
		if strings.Contains(lowerURL, sensitiveField+"=") {
			// Found sensitive parameter, redact the value
			url = redactQueryParam(url, sensitiveField)
		}
	}
	
	return url
}

// redactQueryParam redacts the value of a query parameter.
func redactQueryParam(url, param string) string {
	// Simple replacement - in production, use proper URL parsing
	lowerURL := strings.ToLower(url)
	lowerParam := strings.ToLower(param)
	
	if idx := strings.Index(lowerURL, lowerParam+"="); idx != -1 {
		startIdx := idx + len(lowerParam) + 1
		endIdx := strings.IndexAny(url[startIdx:], "&")
		
		if endIdx == -1 {
			// Last parameter
			return url[:startIdx] + redactedValue
		}
		
		// Middle parameter
		return url[:startIdx] + redactedValue + url[startIdx+endIdx:]
	}
	
	return url
}
