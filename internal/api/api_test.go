package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"godb/internal/database"
)

func setupAPITestServer(t *testing.T) (*Server, *http.ServeMux, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "godb_api_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mgr, err := database.NewManager(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("Failed to create manager: %v", err)
	}

	server := NewServer(mgr)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	cleanup := func() {
		_ = mgr.Close()
		_ = os.RemoveAll(tempDir)
	}
	return server, mux, cleanup
}

func TestAPIEndpoints(t *testing.T) {
	_, mux, cleanup := setupAPITestServer(t)
	defer cleanup()

	// 1. Load sample database
	req := httptest.NewRequest("POST", "/api/databases/sample", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for sample load, got %d: %s", w.Code, w.Body.String())
	}

	// 2. GET Schema
	req = httptest.NewRequest("GET", "/api/schema", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for schema, got %d", w.Code)
	}

	var schemaResp struct {
		Success bool `json:"success"`
		Data    struct {
			Database string                `json:"database"`
			Schema   database.SchemaGraph `json:"schema"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&schemaResp); err != nil {
		t.Fatalf("Failed to decode schema response: %v", err)
	}
	if len(schemaResp.Data.Schema.Tables) < 4 {
		t.Fatalf("Expected at least 4 tables, got %d", len(schemaResp.Data.Schema.Tables))
	}

	// 3. GET Customers Data
	req = httptest.NewRequest("GET", "/api/data/customers?page=1&page_size=10", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for data get, got %d", w.Code)
	}

	// 4. POST Insert Row into customers
	newCust := map[string]interface{}{
		"data": map[string]interface{}{
			"name":    "Grace Hopper",
			"email":   "grace.hopper@navy.mil",
			"city":    "Arlington",
			"country": "USA",
		},
	}
	bodyBytes, _ := json.Marshal(newCust)
	req = httptest.NewRequest("POST", "/api/data/customers/row", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201 for insert row, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Query Builder execute
	qbReq := database.QueryBuilderRequest{
		BaseTable: "customers",
		Columns:   []string{"id", "name", "email"},
		Conditions: []database.WhereCondition{
			{Column: "name", Operator: "LIKE", Value: "%Grace%"},
		},
	}
	qbBytes, _ := json.Marshal(qbReq)
	req = httptest.NewRequest("POST", "/api/query/build", bytes.NewReader(qbBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for query build, got %d: %s", w.Code, w.Body.String())
	}

	var qbResp struct {
		Success bool                 `json:"success"`
		Data    database.QueryResult `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&qbResp); err != nil {
		t.Fatalf("Failed to decode query builder response: %v", err)
	}
	if qbResp.Data.RowCount != 1 {
		t.Fatalf("Expected 1 result for Grace, got %d", qbResp.Data.RowCount)
	}

	// 6. Raw SQL Execute
	sqlBody := map[string]string{
		"query": "SELECT COUNT(*) as total FROM customers",
	}
	sqlBytes, _ := json.Marshal(sqlBody)
	req = httptest.NewRequest("POST", "/api/query/execute", bytes.NewReader(sqlBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for raw SQL, got %d: %s", w.Code, w.Body.String())
	}
}
