package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"godb/internal/database"
)

type Server struct {
	dbMgr *database.Manager
}

func NewServer(dbMgr *database.Manager) *Server {
	return &Server{
		dbMgr: dbMgr,
	}
}

// RegisterRoutes registers all REST API endpoints on the given ServeMux
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Database endpoints
	mux.HandleFunc("GET /api/databases", s.handleListDatabases)
	mux.HandleFunc("POST /api/databases", s.handleCreateDatabase)
	mux.HandleFunc("POST /api/databases/switch", s.handleSwitchDatabase)
	mux.HandleFunc("DELETE /api/databases/{name}", s.handleDeleteDatabase)
	mux.HandleFunc("POST /api/databases/sample", s.handleLoadSample)

	// Schema & Table endpoints
	mux.HandleFunc("GET /api/schema", s.handleGetSchema)
	mux.HandleFunc("POST /api/tables", s.handleCreateTable)
	mux.HandleFunc("DELETE /api/tables/{name}", s.handleDropTable)
	mux.HandleFunc("POST /api/tables/{name}/columns", s.handleAddColumn)
	mux.HandleFunc("POST /api/tables/link", s.handleLinkTables)

	// CRUD Data endpoints
	mux.HandleFunc("GET /api/data/{table}", s.handleGetRows)
	mux.HandleFunc("POST /api/data/{table}/row", s.handleInsertRow)
	mux.HandleFunc("PUT /api/data/{table}/row", s.handleUpdateRow)
	mux.HandleFunc("DELETE /api/data/{table}/row", s.handleDeleteRow)
	mux.HandleFunc("GET /api/data/{table}/fk-options", s.handleGetFKOptions)

	// SQL Query Builder & Execution endpoints
	mux.HandleFunc("POST /api/query/preview", s.handlePreviewQuery)
	mux.HandleFunc("POST /api/query/build", s.handleBuildAndExecuteQuery)
	mux.HandleFunc("POST /api/query/execute", s.handleExecuteRawSQL)
}

// Databases handlers
func (s *Server) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	dbs, err := s.dbMgr.ListDatabases()
	if err != nil {
		JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, active := s.dbMgr.GetDB()
	JSONResponse(w, http.StatusOK, map[string]interface{}{
		"databases": dbs,
		"active":    active,
	})
}

func (s *Server) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.dbMgr.CreateDatabase(body.Name); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, active := s.dbMgr.GetDB()
	JSONMessage(w, http.StatusOK, "Database created and activated", map[string]string{"active": active})
}

func (s *Server) handleSwitchDatabase(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.dbMgr.SwitchDatabase(body.Name); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, active := s.dbMgr.GetDB()
	JSONMessage(w, http.StatusOK, "Switched active database", map[string]string{"active": active})
}

func (s *Server) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		JSONError(w, http.StatusBadRequest, "database name is required")
		return
	}
	if err := s.dbMgr.DeleteDatabase(name); err != nil {
		JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Database deleted", nil)
}

func (s *Server) handleLoadSample(w http.ResponseWriter, r *http.Request) {
	db, active := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	if err := database.LoadSampleEcommerceDB(db); err != nil {
		JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Sample e-commerce database loaded into "+active, nil)
}

// Schema & Tables handlers
func (s *Server) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	db, active := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	schema, err := database.GetSchema(db)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSONResponse(w, http.StatusOK, map[string]interface{}{
		"database": active,
		"schema":   schema,
	})
}

func (s *Server) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	var req database.CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := database.CreateTable(db, req); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusCreated, "Table '"+req.Name+"' created successfully", nil)
}

func (s *Server) handleDropTable(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	tableName := r.PathValue("name")
	if tableName == "" {
		JSONError(w, http.StatusBadRequest, "table name is required")
		return
	}
	if err := database.DropTable(db, tableName); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Table '"+tableName+"' dropped successfully", nil)
}

func (s *Server) handleAddColumn(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	tableName := r.PathValue("name")
	var col database.ColumnDef
	if err := json.NewDecoder(r.Body).Decode(&col); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := database.AddColumn(db, tableName, col); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Column '"+col.Name+"' added to '"+tableName+"'", nil)
}

func (s *Server) handleLinkTables(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	var req database.LinkTablesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := database.LinkTables(db, req); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Foreign key link established successfully", nil)
}

// CRUD Data handlers
func (s *Server) handleGetRows(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	tableName := r.PathValue("table")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	search := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	res, err := database.GetRows(db, tableName, page, pageSize, search, sortBy, sortOrder)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONResponse(w, http.StatusOK, res)
}

func (s *Server) handleInsertRow(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	tableName := r.PathValue("table")
	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lastID, err := database.InsertRow(db, tableName, body.Data)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusCreated, "Record inserted successfully", map[string]interface{}{
		"inserted_id": lastID,
	})
}

func (s *Server) handleUpdateRow(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	tableName := r.PathValue("table")
	var body struct {
		PK   map[string]interface{} `json:"pk"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := database.UpdateRow(db, tableName, body.PK, body.Data); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Record updated successfully", nil)
}

func (s *Server) handleDeleteRow(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	tableName := r.PathValue("table")
	var body struct {
		PK map[string]interface{} `json:"pk"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := database.DeleteRow(db, tableName, body.PK); err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONMessage(w, http.StatusOK, "Record deleted successfully", nil)
}

func (s *Server) handleGetFKOptions(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	refTable := r.URL.Query().Get("ref_table")
	refColumn := r.URL.Query().Get("ref_column")
	if refTable == "" || refColumn == "" {
		JSONError(w, http.StatusBadRequest, "ref_table and ref_column are required")
		return
	}

	options, err := database.GetForeignKeyOptions(db, refTable, refColumn)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONResponse(w, http.StatusOK, options)
}

// SQL Query Builder handlers
func (s *Server) handlePreviewQuery(w http.ResponseWriter, r *http.Request) {
	var req database.QueryBuilderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	querySQL, _, err := database.BuildSQL(req)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONResponse(w, http.StatusOK, map[string]string{"sql": querySQL})
}

func (s *Server) handleBuildAndExecuteQuery(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	var req database.QueryBuilderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	querySQL, args, err := database.BuildSQL(req)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := database.ExecuteSQL(db, querySQL, args...)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result.GeneratedSQL = querySQL
	JSONResponse(w, http.StatusOK, result)
}

func (s *Server) handleExecuteRawSQL(w http.ResponseWriter, r *http.Request) {
	db, _ := s.dbMgr.GetDB()
	if db == nil {
		JSONError(w, http.StatusBadRequest, "no active database")
		return
	}
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.Query) == "" {
		JSONError(w, http.StatusBadRequest, "query cannot be empty")
		return
	}

	result, err := database.ExecuteSQL(db, body.Query)
	if err != nil {
		JSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	JSONResponse(w, http.StatusOK, result)
}
