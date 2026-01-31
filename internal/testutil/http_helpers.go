package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// ReadJSONResponse reads and unmarshals a JSON response from a ResponseRecorder.
func ReadJSONResponse(t interface {
	Errorf(format string, args ...interface{})
	FailNow()
}, w *httptest.ResponseRecorder, v interface{}) {
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
		t.FailNow()
	}

	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Errorf("failed to decode JSON response: %v", err)
		t.FailNow()
	}
}

// ReadErrorResponse reads an error response from a ResponseRecorder.
func ReadErrorResponse(t interface {
	Errorf(format string, args ...interface{})
	FailNow()
}, w *httptest.ResponseRecorder) map[string]interface{} {
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode error response: %v", err)
		t.FailNow()
	}
	return response
}

// CreateRequest creates an HTTP request with optional body and headers.
func CreateRequest(method, path string, body interface{}, headers map[string]string) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(jsonData)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req
}
