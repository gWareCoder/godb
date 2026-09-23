package database

import (
	"database/sql"
	"fmt"
	"strings"
)

type ColumnDef struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"` // TEXT, INTEGER, REAL, BOOLEAN, DATETIME, BLOB
	PrimaryKey    bool    `json:"primary_key"`
	AutoIncrement bool    `json:"auto_increment"`
	NotNull       bool    `json:"not_null"`
	Unique        bool    `json:"unique"`
	DefaultValue  *string `json:"default_value"`
}

type ForeignKeyDef struct {
	ID        int    `json:"id"`
	Column    string `json:"column"`     // Column in this table
	RefTable  string `json:"ref_table"`  // Referenced table
	RefColumn string `json:"ref_column"` // Referenced column
	OnDelete  string `json:"on_delete"`  // CASCADE, SET NULL, RESTRICT, NO ACTION
	OnUpdate  string `json:"on_update"`  // CASCADE, SET NULL, RESTRICT, NO ACTION
}

type TableSchema struct {
	Name        string          `json:"name"`
	Columns     []ColumnDef     `json:"columns"`
	PrimaryKeys []string        `json:"primary_keys"`
	ForeignKeys []ForeignKeyDef `json:"foreign_keys"`
	RowCount    int64           `json:"row_count"`
}

type RelationEdge struct {
	FromTable  string `json:"from_table"`
	FromColumn string `json:"from_column"`
	ToTable    string `json:"to_table"`
	ToColumn   string `json:"to_column"`
	OnDelete   string `json:"on_delete"`
	OnUpdate   string `json:"on_update"`
}

type SchemaGraph struct {
	Tables    []TableSchema  `json:"tables"`
	Relations []RelationEdge `json:"relations"`
}

type CreateTableRequest struct {
	Name        string          `json:"name"`
	Columns     []ColumnDef     `json:"columns"`
	ForeignKeys []ForeignKeyDef `json:"foreign_keys"`
}

type LinkTablesRequest struct {
	SourceTable  string `json:"source_table"`
	SourceColumn string `json:"source_column"`
	TargetTable  string `json:"target_table"`
	TargetColumn string `json:"target_column"`
	OnDelete     string `json:"on_delete"` // CASCADE, SET NULL, RESTRICT, NO ACTION
	OnUpdate     string `json:"on_update"`
}

// sanitizeIdentifier escapes double quotes for SQL identifiers
func sanitizeIdentifier(name string) string {
	return strings.ReplaceAll(name, "\"", "\"\"")
}

// GetSchema returns the complete schema including columns, constraints, and relationships
func GetSchema(db *sql.DB) (*SchemaGraph, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}

	graph := &SchemaGraph{
		Tables:    make([]TableSchema, 0, len(tableNames)),
		Relations: make([]RelationEdge, 0),
	}

	for _, tblName := range tableNames {
		tableSchema, err := inspectTable(db, tblName)
		if err != nil {
			return nil, fmt.Errorf("error inspecting table %s: %w", tblName, err)
		}
		graph.Tables = append(graph.Tables, *tableSchema)

		// Collect relations for graph
		for _, fk := range tableSchema.ForeignKeys {
			graph.Relations = append(graph.Relations, RelationEdge{
				FromTable:  tblName,
				FromColumn: fk.Column,
				ToTable:    fk.RefTable,
				ToColumn:   fk.RefColumn,
				OnDelete:   fk.OnDelete,
				OnUpdate:   fk.OnUpdate,
			})
		}
	}

	return graph, nil
}

func inspectTable(db *sql.DB, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		Name:        tableName,
		Columns:     make([]ColumnDef, 0),
		PrimaryKeys: make([]string, 0),
		ForeignKeys: make([]ForeignKeyDef, 0),
	}

	// 1. Get Row Count
	var count int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", sanitizeIdentifier(tableName))
	if err := db.QueryRow(countQuery).Scan(&count); err == nil {
		schema.RowCount = count
	}

	// 2. Query table_info
	infoQuery := fmt.Sprintf("PRAGMA table_info(\"%s\")", sanitizeIdentifier(tableName))
	infoRows, err := db.Query(infoQuery)
	if err != nil {
		return nil, err
	}
	defer infoRows.Close()

	// cid, name, type, notnull, dflt_value, pk
	for infoRows.Next() {
		var (
			cid       int
			colName   string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := infoRows.Scan(&cid, &colName, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		var dflt *string
		if dfltValue.Valid {
			dflt = &dfltValue.String
		}

		isPK := pk > 0
		if isPK {
			schema.PrimaryKeys = append(schema.PrimaryKeys, colName)
		}

		schema.Columns = append(schema.Columns, ColumnDef{
			Name:         colName,
			Type:         strings.ToUpper(colType),
			PrimaryKey:   isPK,
			NotNull:      notNull == 1,
			DefaultValue: dflt,
		})
	}

	// Check for AUTOINCREMENT in sqlite_master sql definition
	var sqlDef string
	_ = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name = ?", tableName).Scan(&sqlDef)
	upperSQL := strings.ToUpper(sqlDef)
	for i := range schema.Columns {
		col := &schema.Columns[i]
		if col.PrimaryKey {
			pattern := fmt.Sprintf("\"%s\" INTEGER PRIMARY KEY AUTOINCREMENT", strings.ToUpper(col.Name))
			pattern2 := fmt.Sprintf("%s INTEGER PRIMARY KEY AUTOINCREMENT", strings.ToUpper(col.Name))
			if strings.Contains(upperSQL, pattern) || strings.Contains(upperSQL, pattern2) || strings.Contains(upperSQL, "AUTOINCREMENT") {
				col.AutoIncrement = true
			}
		}
	}

	// 3. Query foreign_key_list
	fkQuery := fmt.Sprintf("PRAGMA foreign_key_list(\"%s\")", sanitizeIdentifier(tableName))
	fkRows, err := db.Query(fkQuery)
	if err == nil {
		defer fkRows.Close()
		// id, seq, table, from, to, on_update, on_delete, match
		for fkRows.Next() {
			var (
				id       int
				seq      int
				refTable string
				fromCol  string
				toCol    string
				onUpdate string
				onDelete string
				match    string
			)
			if err := fkRows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err == nil {
				schema.ForeignKeys = append(schema.ForeignKeys, ForeignKeyDef{
					ID:        id,
					Column:    fromCol,
					RefTable:  refTable,
					RefColumn: toCol,
					OnDelete:  onDelete,
					OnUpdate:  onUpdate,
				})
			}
		}
	}

	return schema, nil
}

// CreateTable creates a new table with columns and foreign key constraints
func CreateTable(db *sql.DB, req CreateTableRequest) error {
	tableName := strings.TrimSpace(req.Name)
	if tableName == "" {
		return fmt.Errorf("table name is required")
	}
	if len(req.Columns) == 0 {
		return fmt.Errorf("table must have at least one column")
	}

	var colClauses []string
	var pkCount int
	for _, col := range req.Columns {
		if col.PrimaryKey {
			pkCount++
		}
	}

	for _, col := range req.Columns {
		colName := strings.TrimSpace(col.Name)
		if colName == "" {
			return fmt.Errorf("column name cannot be empty")
		}

		colType := strings.ToUpper(strings.TrimSpace(col.Type))
		if colType == "" {
			colType = "TEXT"
		}

		clause := fmt.Sprintf("\"%s\" %s", sanitizeIdentifier(colName), colType)

		// SQLite single primary key with autoincrement
		if col.PrimaryKey && pkCount == 1 {
			clause += " PRIMARY KEY"
			if col.AutoIncrement && (colType == "INTEGER" || colType == "INT") {
				clause += " AUTOINCREMENT"
			}
		}

		if col.NotNull && !(col.PrimaryKey && pkCount == 1) {
			clause += " NOT NULL"
		}

		if col.Unique && !(col.PrimaryKey && pkCount == 1) {
			clause += " UNIQUE"
		}

		if col.DefaultValue != nil && *col.DefaultValue != "" {
			clause += fmt.Sprintf(" DEFAULT %s", *col.DefaultValue)
		}

		colClauses = append(colClauses, clause)
	}

	// Composite primary key if multiple PK columns
	if pkCount > 1 {
		var pkCols []string
		for _, col := range req.Columns {
			if col.PrimaryKey {
				pkCols = append(pkCols, fmt.Sprintf("\"%s\"", sanitizeIdentifier(col.Name)))
			}
		}
		colClauses = append(colClauses, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	// Foreign Keys
	for _, fk := range req.ForeignKeys {
		if fk.Column == "" || fk.RefTable == "" || fk.RefColumn == "" {
			continue
		}
		fkClause := fmt.Sprintf(
			"FOREIGN KEY (\"%s\") REFERENCES \"%s\" (\"%s\")",
			sanitizeIdentifier(fk.Column),
			sanitizeIdentifier(fk.RefTable),
			sanitizeIdentifier(fk.RefColumn),
		)
		if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
			fkClause += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
			fkClause += " ON UPDATE " + fk.OnUpdate
		}
		colClauses = append(colClauses, fkClause)
	}

	query := fmt.Sprintf("CREATE TABLE \"%s\" (\n  %s\n);",
		sanitizeIdentifier(tableName),
		strings.Join(colClauses, ",\n  "),
	)

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w\nQuery: %s", err, query)
	}

	return nil
}

// DropTable drops a table
func DropTable(db *sql.DB, tableName string) error {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return fmt.Errorf("table name is required")
	}

	query := fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", sanitizeIdentifier(tableName))
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	return nil
}

// AddColumn adds a column to an existing table
func AddColumn(db *sql.DB, tableName string, col ColumnDef) error {
	colName := strings.TrimSpace(col.Name)
	if colName == "" {
		return fmt.Errorf("column name is required")
	}
	colType := strings.ToUpper(strings.TrimSpace(col.Type))
	if colType == "" {
		colType = "TEXT"
	}

	clause := fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"%s\" %s",
		sanitizeIdentifier(tableName),
		sanitizeIdentifier(colName),
		colType,
	)

	if col.NotNull && col.DefaultValue != nil {
		clause += fmt.Sprintf(" NOT NULL DEFAULT %s", *col.DefaultValue)
	} else if col.DefaultValue != nil && *col.DefaultValue != "" {
		clause += fmt.Sprintf(" DEFAULT %s", *col.DefaultValue)
	}

	_, err := db.Exec(clause)
	if err != nil {
		return fmt.Errorf("failed to add column: %w", err)
	}
	return nil
}

// LinkTables adds a foreign key relationship between two existing tables using a safe table recreation migration
func LinkTables(db *sql.DB, req LinkTablesRequest) error {
	req.SourceTable = strings.TrimSpace(req.SourceTable)
	req.SourceColumn = strings.TrimSpace(req.SourceColumn)
	req.TargetTable = strings.TrimSpace(req.TargetTable)
	req.TargetColumn = strings.TrimSpace(req.TargetColumn)

	if req.SourceTable == "" || req.SourceColumn == "" || req.TargetTable == "" || req.TargetColumn == "" {
		return fmt.Errorf("source table/column and target table/column are all required")
	}

	// 1. Inspect source table
	schema, err := inspectTable(db, req.SourceTable)
	if err != nil {
		return fmt.Errorf("failed to inspect source table: %w", err)
	}

	// Verify source column exists
	var srcColFound bool
	for _, col := range schema.Columns {
		if col.Name == req.SourceColumn {
			srcColFound = true
			break
		}
	}
	if !srcColFound {
		return fmt.Errorf("column %s does not exist in table %s", req.SourceColumn, req.SourceTable)
	}

	// Check if already linked
	for _, fk := range schema.ForeignKeys {
		if fk.Column == req.SourceColumn && fk.RefTable == req.TargetTable && fk.RefColumn == req.TargetColumn {
			return fmt.Errorf("table %s.%s is already linked to %s.%s", req.SourceTable, req.SourceColumn, req.TargetTable, req.TargetColumn)
		}
	}

	// Add new foreign key to schema
	onDel := req.OnDelete
	if onDel == "" {
		onDel = "CASCADE"
	}
	onUpd := req.OnUpdate
	if onUpd == "" {
		onUpd = "CASCADE"
	}

	newFKs := append(schema.ForeignKeys, ForeignKeyDef{
		Column:    req.SourceColumn,
		RefTable:  req.TargetTable,
		RefColumn: req.TargetColumn,
		OnDelete:  onDel,
		OnUpdate:  onUpd,
	})

	// Begin migration transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Disable foreign keys check temporarily during migration
	if _, err := tx.Exec("PRAGMA foreign_keys = OFF;"); err != nil {
		return err
	}

	tempTable := fmt.Sprintf("_temp_mig_%s", req.SourceTable)
	createReq := CreateTableRequest{
		Name:        tempTable,
		Columns:     schema.Columns,
		ForeignKeys: newFKs,
	}

	// Create temp table inside transaction
	var colClauses []string
	var pkCols []string
	for _, col := range createReq.Columns {
		clause := fmt.Sprintf("\"%s\" %s", sanitizeIdentifier(col.Name), col.Type)
		if col.PrimaryKey && len(schema.PrimaryKeys) == 1 {
			clause += " PRIMARY KEY"
			if col.AutoIncrement && (col.Type == "INTEGER" || col.Type == "INT") {
				clause += " AUTOINCREMENT"
			}
		}
		if col.NotNull && !(col.PrimaryKey && len(schema.PrimaryKeys) == 1) {
			clause += " NOT NULL"
		}
		if col.DefaultValue != nil && *col.DefaultValue != "" {
			clause += fmt.Sprintf(" DEFAULT %s", *col.DefaultValue)
		}
		colClauses = append(colClauses, clause)
		if col.PrimaryKey {
			pkCols = append(pkCols, fmt.Sprintf("\"%s\"", sanitizeIdentifier(col.Name)))
		}
	}

	if len(schema.PrimaryKeys) > 1 {
		colClauses = append(colClauses, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	for _, fk := range createReq.ForeignKeys {
		fkClause := fmt.Sprintf(
			"FOREIGN KEY (\"%s\") REFERENCES \"%s\" (\"%s\")",
			sanitizeIdentifier(fk.Column),
			sanitizeIdentifier(fk.RefTable),
			sanitizeIdentifier(fk.RefColumn),
		)
		if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
			fkClause += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
			fkClause += " ON UPDATE " + fk.OnUpdate
		}
		colClauses = append(colClauses, fkClause)
	}

	createTempSQL := fmt.Sprintf("CREATE TABLE \"%s\" (\n  %s\n);", tempTable, strings.Join(colClauses, ",\n  "))
	if _, err := tx.Exec(createTempSQL); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// Copy column data
	var colNames []string
	for _, col := range schema.Columns {
		colNames = append(colNames, fmt.Sprintf("\"%s\"", sanitizeIdentifier(col.Name)))
	}
	colsJoined := strings.Join(colNames, ", ")
	copySQL := fmt.Sprintf("INSERT INTO \"%s\" (%s) SELECT %s FROM \"%s\";", tempTable, colsJoined, colsJoined, sanitizeIdentifier(req.SourceTable))
	if _, err := tx.Exec(copySQL); err != nil {
		return fmt.Errorf("failed to copy data to migration table: %w", err)
	}

	// Drop old table
	dropSQL := fmt.Sprintf("DROP TABLE \"%s\";", sanitizeIdentifier(req.SourceTable))
	if _, err := tx.Exec(dropSQL); err != nil {
		return fmt.Errorf("failed to drop old table during migration: %w", err)
	}

	// Rename temp table to source table
	renameSQL := fmt.Sprintf("ALTER TABLE \"%s\" RENAME TO \"%s\";", tempTable, sanitizeIdentifier(req.SourceTable))
	if _, err := tx.Exec(renameSQL); err != nil {
		return fmt.Errorf("failed to rename migration table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Re-enable foreign keys
	_, _ = db.Exec("PRAGMA foreign_keys = ON;")
	return nil
}
