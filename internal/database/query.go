package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type JoinDef struct {
	Type       string `json:"type"` // INNER, LEFT, CROSS
	Table      string `json:"table"`
	FromColumn string `json:"from_column"` // e.g. base_table.customer_id
	ToColumn   string `json:"to_column"`   // e.g. joined_table.id
	CustomOn   string `json:"custom_on"`   // optional custom ON clause
}

type WhereCondition struct {
	Column     string `json:"column"`
	Operator   string `json:"operator"` // =, !=, >, >=, <, <=, LIKE, NOT LIKE, IS NULL, IS NOT NULL
	Value      string `json:"value"`
	Combinator string `json:"combinator"` // AND, OR
}

type QueryBuilderRequest struct {
	BaseTable  string           `json:"base_table"`
	Columns    []string         `json:"columns"`
	Joins      []JoinDef        `json:"joins"`
	Conditions []WhereCondition `json:"conditions"`
	OrderBy    string           `json:"order_by"`
	OrderDir   string           `json:"order_dir"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

type QueryResult struct {
	Columns      []string                 `json:"columns"`
	Rows         []map[string]interface{} `json:"rows"`
	RowCount     int                      `json:"row_count"`
	RowsAffected int64                    `json:"rows_affected"`
	LastInsertID int64                    `json:"last_insert_id"`
	ExecutionMs  float64                  `json:"execution_ms"`
	IsSelect     bool                     `json:"is_select"`
	Message      string                   `json:"message"`
	GeneratedSQL string                   `json:"generated_sql,omitempty"`
}

// BuildSQL constructs SQL statement from QueryBuilderRequest
func BuildSQL(req QueryBuilderRequest) (string, []interface{}, error) {
	req.BaseTable = strings.TrimSpace(req.BaseTable)
	if req.BaseTable == "" {
		return "", nil, fmt.Errorf("base table is required")
	}

	var colsPart string
	if len(req.Columns) == 0 {
		colsPart = "*"
	} else {
		var safeCols []string
		for _, col := range req.Columns {
			col = strings.TrimSpace(col)
			if col == "" {
				continue
			}
			// If column contains a table prefix e.g. "customers.name" or function
			if strings.Contains(col, ".") || strings.Contains(col, "(") || col == "*" {
				safeCols = append(safeCols, col)
			} else {
				safeCols = append(safeCols, fmt.Sprintf("\"%s\"", sanitizeIdentifier(col)))
			}
		}
		if len(safeCols) == 0 {
			colsPart = "*"
		} else {
			colsPart = strings.Join(safeCols, ", ")
		}
	}

	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString(fmt.Sprintf("SELECT %s FROM \"%s\"", colsPart, sanitizeIdentifier(req.BaseTable)))

	// Joins
	for _, j := range req.Joins {
		jTable := strings.TrimSpace(j.Table)
		if jTable == "" {
			continue
		}
		jType := strings.ToUpper(strings.TrimSpace(j.Type))
		if jType == "" {
			jType = "INNER"
		}

		var onClause string
		if strings.TrimSpace(j.CustomOn) != "" {
			onClause = j.CustomOn
		} else if j.FromColumn != "" && j.ToColumn != "" {
			onClause = fmt.Sprintf("%s = %s", j.FromColumn, j.ToColumn)
		} else {
			continue
		}

		sqlBuilder.WriteString(fmt.Sprintf(" %s JOIN \"%s\" ON %s", jType, sanitizeIdentifier(jTable), onClause))
	}

	var args []interface{}

	// WHERE conditions
	if len(req.Conditions) > 0 {
		var whereParts []string
		for i, cond := range req.Conditions {
			col := strings.TrimSpace(cond.Column)
			if col == "" {
				continue
			}
			op := strings.ToUpper(strings.TrimSpace(cond.Operator))
			if op == "" {
				op = "="
			}

			combinator := strings.ToUpper(strings.TrimSpace(cond.Combinator))
			if combinator == "" {
				combinator = "AND"
			}

			// Format column identifier if needed
			colExpr := col
			if !strings.Contains(col, ".") && !strings.Contains(col, "(") {
				colExpr = fmt.Sprintf("\"%s\"", sanitizeIdentifier(col))
			}

			switch op {
			case "IS NULL", "IS NOT NULL":
				whereParts = appendCondition(whereParts, fmt.Sprintf("%s %s", colExpr, op), combinator, i)
			case "LIKE", "NOT LIKE":
				whereParts = appendCondition(whereParts, fmt.Sprintf("%s %s ?", colExpr, op), combinator, i)
				args = append(args, cond.Value)
			case "IN":
				// Handle comma-separated list
				items := strings.Split(cond.Value, ",")
				var inPlaceholders []string
				for _, item := range items {
					trimmed := strings.TrimSpace(item)
					inPlaceholders = append(inPlaceholders, "?")
					args = append(args, trimmed)
				}
				whereParts = appendCondition(whereParts, fmt.Sprintf("%s IN (%s)", colExpr, strings.Join(inPlaceholders, ", ")), combinator, i)
			default:
				// Standard comparison (=, !=, >, >=, <, <=)
				whereParts = appendCondition(whereParts, fmt.Sprintf("%s %s ?", colExpr, op), combinator, i)
				args = append(args, cond.Value)
			}
		}

		if len(whereParts) > 0 {
			sqlBuilder.WriteString(" WHERE ")
			sqlBuilder.WriteString(strings.Join(whereParts, " "))
		}
	}

	// ORDER BY
	if strings.TrimSpace(req.OrderBy) != "" {
		orderCol := strings.TrimSpace(req.OrderBy)
		orderDir := "ASC"
		if strings.EqualFold(req.OrderDir, "desc") {
			orderDir = "DESC"
		}
		if !strings.Contains(orderCol, ".") {
			orderCol = fmt.Sprintf("\"%s\"", sanitizeIdentifier(orderCol))
		}
		sqlBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderCol, orderDir))
	}

	// LIMIT & OFFSET
	if req.Limit > 0 {
		sqlBuilder.WriteString(fmt.Sprintf(" LIMIT %d", req.Limit))
		if req.Offset > 0 {
			sqlBuilder.WriteString(fmt.Sprintf(" OFFSET %d", req.Offset))
		}
	}

	return sqlBuilder.String(), args, nil
}

func appendCondition(parts []string, expr, combinator string, index int) []string {
	if index == 0 {
		return append(parts, expr)
	}
	return append(parts, combinator, expr)
}

// ExecuteSQL executes raw SQL and returns structured results with execution metrics
func ExecuteSQL(db *sql.DB, query string, args ...interface{}) (*QueryResult, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	upper := strings.ToUpper(trimmed)
	isSelect := strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "PRAGMA") || strings.HasPrefix(upper, "EXPLAIN")

	start := time.Now()

	if isSelect {
		rows, err := db.Query(trimmed, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		resultRows := make([]map[string]interface{}, 0)
		for rows.Next() {
			vals := make([]interface{}, len(columns))
			ptrs := make([]interface{}, len(columns))
			for i := range vals {
				ptrs[i] = &vals[i]
			}

			if err := rows.Scan(ptrs...); err != nil {
				return nil, err
			}

			rowMap := make(map[string]interface{})
			for i, col := range columns {
				val := vals[i]
				if b, ok := val.([]byte); ok {
					rowMap[col] = string(b)
				} else {
					rowMap[col] = val
				}
			}
			resultRows = append(resultRows, rowMap)
		}

		duration := float64(time.Since(start).Microseconds()) / 1000.0

		return &QueryResult{
			Columns:      columns,
			Rows:         resultRows,
			RowCount:     len(resultRows),
			ExecutionMs:  duration,
			IsSelect:     true,
			Message:      fmt.Sprintf("%d rows returned in %.2f ms", len(resultRows), duration),
			GeneratedSQL: query,
		}, nil
	}

	// Exec modification or DDL query
	res, err := db.Exec(trimmed, args...)
	if err != nil {
		return nil, err
	}

	duration := float64(time.Since(start).Microseconds()) / 1000.0
	rowsAff, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()

	msg := fmt.Sprintf("Query executed successfully in %.2f ms", duration)
	if rowsAff > 0 {
		msg += fmt.Sprintf(" (%d rows affected)", rowsAff)
	}

	return &QueryResult{
		RowsAffected: rowsAff,
		LastInsertID: lastID,
		ExecutionMs:  duration,
		IsSelect:     false,
		Message:      msg,
		GeneratedSQL: query,
	}, nil
}
