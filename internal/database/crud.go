package database

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

type TableRowResult struct {
	Columns    []ColumnDef              `json:"columns"`
	Rows       []map[string]interface{} `json:"rows"`
	TotalRows  int64                    `json:"total_rows"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	TotalPages int                      `json:"total_pages"`
}

type FKOption struct {
	Value interface{} `json:"value"`
	Label string      `json:"label"`
}

// GetRows retrieves paginated rows with optional search and sorting
func GetRows(db *sql.DB, tableName string, page, pageSize int, search, sortBy, sortOrder string) (*TableRowResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 25
	}

	tableSchema, err := inspectTable(db, tableName)
	if err != nil {
		return nil, err
	}

	whereClauses := []string{}
	args := []interface{}{}

	if strings.TrimSpace(search) != "" {
		var searchConditions []string
		for _, col := range tableSchema.Columns {
			searchConditions = append(searchConditions, fmt.Sprintf("CAST(\"%s\" AS TEXT) LIKE ?", sanitizeIdentifier(col.Name)))
			args = append(args, "%"+search+"%")
		}
		if len(searchConditions) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(searchConditions, " OR ")+")")
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"%s", sanitizeIdentifier(tableName), whereSQL)
	var totalRows int64
	if err := db.QueryRow(countQuery, args...).Scan(&totalRows); err != nil {
		return nil, fmt.Errorf("failed to count rows: %w", err)
	}

	// Validate sortBy
	var validSortCol string
	if sortBy != "" {
		for _, col := range tableSchema.Columns {
			if strings.EqualFold(col.Name, sortBy) {
				validSortCol = col.Name
				break
			}
		}
	}
	if validSortCol == "" {
		if len(tableSchema.PrimaryKeys) > 0 {
			validSortCol = tableSchema.PrimaryKeys[0]
		} else if len(tableSchema.Columns) > 0 {
			validSortCol = tableSchema.Columns[0].Name
		}
	}

	orderDir := "ASC"
	if strings.EqualFold(sortOrder, "desc") {
		orderDir = "DESC"
	}

	orderSQL := ""
	if validSortCol != "" {
		orderSQL = fmt.Sprintf(" ORDER BY \"%s\" %s", sanitizeIdentifier(validSortCol), orderDir)
	}

	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf("SELECT * FROM \"%s\"%s%s LIMIT ? OFFSET ?", sanitizeIdentifier(tableName), whereSQL, orderSQL)
	queryArgs := append(args, pageSize, offset)

	rows, err := db.Query(dataQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query rows: %w", err)
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	resultRows := make([]map[string]interface{}, 0)
	for rows.Next() {
		colValues := make([]interface{}, len(colNames))
		colPointers := make([]interface{}, len(colNames))
		for i := range colValues {
			colPointers[i] = &colValues[i]
		}

		if err := rows.Scan(colPointers...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range colNames {
			val := colValues[i]
			// SQLite driver might return []byte for strings/blobs
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		resultRows = append(resultRows, rowMap)
	}

	totalPages := int(math.Ceil(float64(totalRows) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &TableRowResult{
		Columns:    tableSchema.Columns,
		Rows:       resultRows,
		TotalRows:  totalRows,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// InsertRow inserts a new row into the specified table
func InsertRow(db *sql.DB, tableName string, data map[string]interface{}) (interface{}, error) {
	tableSchema, err := inspectTable(db, tableName)
	if err != nil {
		return nil, err
	}

	var cols []string
	var placeholders []string
	var values []interface{}

	for _, col := range tableSchema.Columns {
		val, exists := data[col.Name]
		if !exists {
			continue
		}

		// If autoincrement primary key and empty or nil, skip it
		if col.PrimaryKey && col.AutoIncrement && (val == nil || val == "" || val == float64(0)) {
			continue
		}

		// Handle empty string for nullable fields
		if strVal, ok := val.(string); ok && strVal == "" && !col.NotNull {
			val = nil
		}

		cols = append(cols, fmt.Sprintf("\"%s\"", sanitizeIdentifier(col.Name)))
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	if len(cols) == 0 {
		// Insert default row
		insertSQL := fmt.Sprintf("INSERT INTO \"%s\" DEFAULT VALUES", sanitizeIdentifier(tableName))
		res, err := db.Exec(insertSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to insert record: %w", err)
		}
		lastID, _ := res.LastInsertId()
		return lastID, nil
	}

	insertSQL := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s)",
		sanitizeIdentifier(tableName),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	res, err := db.Exec(insertSQL, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert record: %w", err)
	}

	lastID, _ := res.LastInsertId()
	return lastID, nil
}

// UpdateRow updates a row matching primary keys
func UpdateRow(db *sql.DB, tableName string, pkValues map[string]interface{}, data map[string]interface{}) error {
	tableSchema, err := inspectTable(db, tableName)
	if err != nil {
		return err
	}

	if len(pkValues) == 0 {
		return fmt.Errorf("primary key values are required for update")
	}

	var setClauses []string
	var args []interface{}

	for _, col := range tableSchema.Columns {
		// Don't update primary keys unless explicitly intended
		if col.PrimaryKey {
			continue
		}

		val, exists := data[col.Name]
		if !exists {
			continue
		}

		if strVal, ok := val.(string); ok && strVal == "" && !col.NotNull {
			val = nil
		}

		setClauses = append(setClauses, fmt.Sprintf("\"%s\" = ?", sanitizeIdentifier(col.Name)))
		args = append(args, val)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no columns to update")
	}

	var whereClauses []string
	for k, v := range pkValues {
		whereClauses = append(whereClauses, fmt.Sprintf("\"%s\" = ?", sanitizeIdentifier(k)))
		args = append(args, v)
	}

	updateSQL := fmt.Sprintf("UPDATE \"%s\" SET %s WHERE %s",
		sanitizeIdentifier(tableName),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)

	res, err := db.Exec(updateSQL, args...)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("no records were updated (record not found)")
	}

	return nil
}

// DeleteRow deletes a row matching primary keys
func DeleteRow(db *sql.DB, tableName string, pkValues map[string]interface{}) error {
	if len(pkValues) == 0 {
		return fmt.Errorf("primary key values are required for delete")
	}

	var whereClauses []string
	var args []interface{}

	for k, v := range pkValues {
		whereClauses = append(whereClauses, fmt.Sprintf("\"%s\" = ?", sanitizeIdentifier(k)))
		args = append(args, v)
	}

	deleteSQL := fmt.Sprintf("DELETE FROM \"%s\" WHERE %s",
		sanitizeIdentifier(tableName),
		strings.Join(whereClauses, " AND "),
	)

	res, err := db.Exec(deleteSQL, args...)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("record not found")
	}

	return nil
}

// GetForeignKeyOptions fetches options from a referenced table for easy selection in forms
func GetForeignKeyOptions(db *sql.DB, refTable, refColumn string) ([]FKOption, error) {
	refSchema, err := inspectTable(db, refTable)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect referenced table %s: %w", refTable, err)
	}

	// Look for human-readable descriptive columns (e.g. name, title, email, username, label, desc)
	preferredNames := []string{"name", "title", "full_name", "first_name", "last_name", "email", "username", "code", "description", "summary"}
	var labelCols []string

	for _, pref := range preferredNames {
		for _, col := range refSchema.Columns {
			if strings.EqualFold(col.Name, pref) {
				labelCols = append(labelCols, col.Name)
				break
			}
		}
		if len(labelCols) >= 2 {
			break
		}
	}

	// Fallback to first non-PK string column if none found
	if len(labelCols) == 0 {
		for _, col := range refSchema.Columns {
			if !col.PrimaryKey && (col.Type == "TEXT" || col.Type == "VARCHAR") {
				labelCols = append(labelCols, col.Name)
				if len(labelCols) >= 2 {
					break
				}
			}
		}
	}

	selectCols := []string{fmt.Sprintf("\"%s\"", sanitizeIdentifier(refColumn))}
	for _, lc := range labelCols {
		if lc != refColumn {
			selectCols = append(selectCols, fmt.Sprintf("\"%s\"", sanitizeIdentifier(lc)))
		}
	}

	query := fmt.Sprintf("SELECT %s FROM \"%s\" ORDER BY \"%s\" ASC LIMIT 100",
		strings.Join(selectCols, ", "),
		sanitizeIdentifier(refTable),
		sanitizeIdentifier(refColumn),
	)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []FKOption
	for rows.Next() {
		colCount := len(selectCols)
		vals := make([]interface{}, colCount)
		ptrs := make([]interface{}, colCount)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		pkVal := vals[0]
		if b, ok := pkVal.([]byte); ok {
			pkVal = string(b)
		}

		labelParts := []string{fmt.Sprintf("#%v", pkVal)}
		for i := 1; i < colCount; i++ {
			if vals[i] != nil {
				s := fmt.Sprintf("%v", vals[i])
				if b, ok := vals[i].([]byte); ok {
					s = string(b)
				}
				if strings.TrimSpace(s) != "" {
					labelParts = append(labelParts, s)
				}
			}
		}

		options = append(options, FKOption{
			Value: pkVal,
			Label: strings.Join(labelParts, " - "),
		})
	}

	return options, nil
}
