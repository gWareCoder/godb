package database

import (
	"os"
	"testing"
)

func setupTestDB(t *testing.T) (*Manager, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "godb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	mgr, err := NewManager(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("Failed to create manager: %v", err)
	}

	cleanup := func() {
		_ = mgr.Close()
		_ = os.RemoveAll(tempDir)
	}
	return mgr, cleanup
}

func TestDatabaseManager(t *testing.T) {
	mgr, cleanup := setupTestDB(t)
	defer cleanup()

	// Initial database should be created
	dbs, err := mgr.ListDatabases()
	if err != nil {
		t.Fatalf("ListDatabases failed: %v", err)
	}
	if len(dbs) == 0 {
		t.Fatalf("Expected at least 1 database, got 0")
	}

	// Create new database
	if err := mgr.CreateDatabase("inventory"); err != nil {
		t.Fatalf("CreateDatabase failed: %v", err)
	}

	_, active := mgr.GetDB()
	if active != "inventory" {
		t.Fatalf("Expected active database to be 'inventory', got '%s'", active)
	}

	// Switch database
	if err := mgr.SwitchDatabase("default"); err != nil {
		t.Fatalf("SwitchDatabase failed: %v", err)
	}
	_, active = mgr.GetDB()
	if active != "default" {
		t.Fatalf("Expected active database to be 'default', got '%s'", active)
	}

	// Delete database
	if err := mgr.DeleteDatabase("inventory"); err != nil {
		t.Fatalf("DeleteDatabase failed: %v", err)
	}

	dbs, _ = mgr.ListDatabases()
	for _, d := range dbs {
		if d.Name == "inventory" {
			t.Fatalf("Expected 'inventory' database to be deleted")
		}
	}
}

func TestCreateTableAndSchema(t *testing.T) {
	mgr, cleanup := setupTestDB(t)
	defer cleanup()

	db, _ := mgr.GetDB()

	// Create Authors table
	authorsReq := CreateTableRequest{
		Name: "authors",
		Columns: []ColumnDef{
			{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "name", Type: "TEXT", NotNull: true},
			{Name: "bio", Type: "TEXT"},
		},
	}
	if err := CreateTable(db, authorsReq); err != nil {
		t.Fatalf("Failed to create authors table: %v", err)
	}

	// Create Books table with foreign key linking to authors
	booksReq := CreateTableRequest{
		Name: "books",
		Columns: []ColumnDef{
			{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "title", Type: "TEXT", NotNull: true},
			{Name: "author_id", Type: "INTEGER", NotNull: true},
			{Name: "price", Type: "REAL"},
		},
		ForeignKeys: []ForeignKeyDef{
			{
				Column:    "author_id",
				RefTable:  "authors",
				RefColumn: "id",
				OnDelete:  "CASCADE",
			},
		},
	}
	if err := CreateTable(db, booksReq); err != nil {
		t.Fatalf("Failed to create books table: %v", err)
	}

	// Inspect schema
	schema, err := GetSchema(db)
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}

	if len(schema.Tables) != 2 {
		t.Fatalf("Expected 2 tables, got %d", len(schema.Tables))
	}

	if len(schema.Relations) != 1 {
		t.Fatalf("Expected 1 relation edge, got %d", len(schema.Relations))
	}

	rel := schema.Relations[0]
	if rel.FromTable != "books" || rel.FromColumn != "author_id" || rel.ToTable != "authors" || rel.ToColumn != "id" {
		t.Fatalf("Unexpected relation: %+v", rel)
	}
}

func TestCRUDAndForeignKeys(t *testing.T) {
	mgr, cleanup := setupTestDB(t)
	defer cleanup()

	db, _ := mgr.GetDB()

	// Setup Authors and Books
	_ = CreateTable(db, CreateTableRequest{
		Name: "authors",
		Columns: []ColumnDef{
			{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "name", Type: "TEXT", NotNull: true},
		},
	})
	_ = CreateTable(db, CreateTableRequest{
		Name: "books",
		Columns: []ColumnDef{
			{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "title", Type: "TEXT", NotNull: true},
			{Name: "author_id", Type: "INTEGER", NotNull: true},
		},
		ForeignKeys: []ForeignKeyDef{
			{Column: "author_id", RefTable: "authors", RefColumn: "id", OnDelete: "CASCADE"},
		},
	})

	// 1. Insert Author
	authorID, err := InsertRow(db, "authors", map[string]interface{}{
		"name": "Isaac Asimov",
	})
	if err != nil {
		t.Fatalf("Insert author failed: %v", err)
	}

	// 2. Insert Book with valid FK
	bookID, err := InsertRow(db, "books", map[string]interface{}{
		"title":     "Foundation",
		"author_id": authorID,
	})
	if err != nil {
		t.Fatalf("Insert book failed: %v", err)
	}
	if bookID == nil {
		t.Fatalf("Expected non-nil bookID")
	}

	// 3. Insert Book with invalid FK should fail due to foreign_keys=ON
	_, err = InsertRow(db, "books", map[string]interface{}{
		"title":     "Phantom Book",
		"author_id": 9999,
	})
	if err == nil {
		t.Fatalf("Expected error when inserting row with non-existent foreign key")
	}

	// 4. FK Options lookup test
	options, err := GetForeignKeyOptions(db, "authors", "id")
	if err != nil {
		t.Fatalf("GetForeignKeyOptions failed: %v", err)
	}
	if len(options) != 1 || options[0].Value != int64(1) {
		t.Fatalf("Expected 1 option for author, got %+v", options)
	}

	// 5. Update book
	err = UpdateRow(db, "books", map[string]interface{}{"id": bookID}, map[string]interface{}{
		"title": "Foundation and Empire",
	})
	if err != nil {
		t.Fatalf("Update book failed: %v", err)
	}

	// 6. Query rows with search
	rowsRes, err := GetRows(db, "books", 1, 10, "Empire", "title", "asc")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rowsRes.Rows) != 1 {
		t.Fatalf("Expected 1 row matching 'Empire', got %d", len(rowsRes.Rows))
	}

	// 7. Cascade delete author should delete book
	err = DeleteRow(db, "authors", map[string]interface{}{"id": authorID})
	if err != nil {
		t.Fatalf("Delete author failed: %v", err)
	}

	booksAfter, _ := GetRows(db, "books", 1, 10, "", "", "")
	if len(booksAfter.Rows) != 0 {
		t.Fatalf("Expected 0 books after cascading delete, got %d", len(booksAfter.Rows))
	}
}

func TestLinkTablesMigration(t *testing.T) {
	mgr, cleanup := setupTestDB(t)
	defer cleanup()

	db, _ := mgr.GetDB()

	// Create users and posts without link first
	_ = CreateTable(db, CreateTableRequest{
		Name: "users",
		Columns: []ColumnDef{
			{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "username", Type: "TEXT", NotNull: true},
		},
	})
	_ = CreateTable(db, CreateTableRequest{
		Name: "posts",
		Columns: []ColumnDef{
			{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
			{Name: "content", Type: "TEXT", NotNull: true},
			{Name: "user_id", Type: "INTEGER"},
		},
	})

	// Add sample post
	_, _ = InsertRow(db, "users", map[string]interface{}{"username": "coder"})
	_, _ = InsertRow(db, "posts", map[string]interface{}{"content": "Hello World", "user_id": 1})

	// Now link posts.user_id -> users.id via LinkTables
	err := LinkTables(db, LinkTablesRequest{
		SourceTable:  "posts",
		SourceColumn: "user_id",
		TargetTable:  "users",
		TargetColumn: "id",
		OnDelete:     "CASCADE",
	})
	if err != nil {
		t.Fatalf("LinkTables failed: %v", err)
	}

	// Verify schema now has the link
	schema, err := GetSchema(db)
	if err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if len(schema.Relations) != 1 {
		t.Fatalf("Expected 1 relation after linking, got %d", len(schema.Relations))
	}
	rel := schema.Relations[0]
	if rel.FromTable != "posts" || rel.FromColumn != "user_id" || rel.ToTable != "users" {
		t.Fatalf("Incorrect relation after link: %+v", rel)
	}

	// Verify existing data preserved
	rows, _ := GetRows(db, "posts", 1, 10, "", "", "")
	if len(rows.Rows) != 1 || rows.Rows[0]["content"] != "Hello World" {
		t.Fatalf("Data was not preserved in migration: %+v", rows.Rows)
	}
}

func TestQueryBuilderAndExecute(t *testing.T) {
	mgr, cleanup := setupTestDB(t)
	defer cleanup()

	db, _ := mgr.GetDB()
	_ = LoadSampleEcommerceDB(db)

	// Visual query builder test
	qbReq := QueryBuilderRequest{
		BaseTable: "orders",
		Columns:   []string{"orders.id", "customers.name", "orders.total_amount", "orders.status"},
		Joins: []JoinDef{
			{
				Type:       "INNER",
				Table:      "customers",
				FromColumn: "orders.customer_id",
				ToColumn:   "customers.id",
			},
		},
		Conditions: []WhereCondition{
			{
				Column:   "orders.total_amount",
				Operator: ">",
				Value:    "150",
			},
		},
		OrderBy:  "orders.total_amount",
		OrderDir: "DESC",
		Limit:    10,
	}

	sqlStr, args, err := BuildSQL(qbReq)
	if err != nil {
		t.Fatalf("BuildSQL failed: %v", err)
	}

	res, err := ExecuteSQL(db, sqlStr, args...)
	if err != nil {
		t.Fatalf("ExecuteSQL failed: %v", err)
	}

	if !res.IsSelect {
		t.Fatalf("Expected IsSelect true")
	}
	if res.RowCount == 0 {
		t.Fatalf("Expected query results, got 0 rows")
	}
}
