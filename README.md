# GoDB - Visual Database & Query Studio

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-gwarecoder-181717?logo=github)](https://github.com/gwarecoder/godb)

**GoDB** is a full-featured, self-contained relational database management application built by [@gwarecoder](https://github.com/gwarecoder) with a high-performance Go backend and a sleek, interactive frontend. It allows users to create databases, visually design and link tables with foreign keys, manage records using intuitive dynamic data entry screens, and construct SQL queries using a visual query builder and raw SQL console.

---

## Key Features

1. **Multi-Database Management**:
   - Create, list, switch, and delete SQLite databases dynamically.
   - Enforces relational foreign key integrity across all connections (`PRAGMA foreign_keys = ON;`).
   - Built-in one-click **Sample E-Commerce Database** (`customers`, `products`, `orders`, `order_items`) for immediate exploration.

2. **Interactive Schema Visualizer**:
   - **ER Diagram Canvas**: Draggable table cards with auto-layout and pan/zoom controls.
   - **Dynamic SVG Connectors**: Smooth cubic bezier curves connecting foreign key columns to their referenced primary key columns with directional arrowheads.
   - **Visual Table Creator**: Modal form to define tables with custom column types (`INTEGER`, `TEXT`, `REAL`, `BOOLEAN`, `DATETIME`, `BLOB`), constraints (`PRIMARY KEY`, `AUTOINCREMENT`, `NOT NULL`, `UNIQUE`, `DEFAULT`), and inline foreign key definitions.
   - **Visual Table Linker**: Add foreign key constraints between existing tables with safe automated table recreation migrations.

3. **User-Friendly Data Entry Screens**:
   - **Smart Foreign Key Dropdowns**: When adding or editing records with foreign keys, the UI automatically inspects referenced tables and displays readable dropdown selectors (e.g. `#1 - Alice Johnson (alice@example.com)`) so you never have to guess or look up raw numeric IDs.
   - **Auto-Generated Inputs**: Appropriate input types (datetime pickers, number steppers, textareas, checkboxes) based on database column schemas.
   - **Spreadsheet-Style Grid**: Instant search filtering, column sorting, pagination controls, and CSV export.
   - **Full CRUD**: Add, edit, and delete records with cascade deletion support.

4. **Visual SQL Query Builder & Console**:
   - **Visual Builder**: Choose base tables, select columns, add filter conditions (`=, !=, >, <, LIKE, IN, IS NULL`), configure `ORDER BY` and `LIMIT`.
   - **1-Click Smart JOINs**: Automatically suggests JOIN clauses based on known foreign key relationships in the schema.
   - **Real-Time SQL Preview**: Live generation of SQL syntax as you customize the builder.
   - **Raw SQL Console**: Monospace code editor with quick SQL templates and execution timing in milliseconds.
   - **Export Results**: Download query results in CSV or JSON format.

---

## Architecture & Technology Stack

- **Backend**: Go (Go 1.22+ standard library `net/http` router, RESTful JSON API).
- **Database Engine**: `modernc.org/sqlite` (100% pure Go SQLite driver, zero CGO/gcc compiler dependencies).
- **Frontend**: Modern vanilla HTML5, CSS3 variables, and ES6 JavaScript (zero runtime external CDN dependencies, fully offline capable).

---

## Quick Start

### 1. Build and Run

To compile and launch the application:

```bash
# Build the binary
./bin/go build -o godb_app main.go

# Start the server on port 8080 (or specify with -port=XXXX)
./godb_app -port=8080
```

Once started, open your web browser at:
```
http://localhost:8080
```

### 2. Run Automated Tests

To run the unit and integration test suite:

```bash
./bin/go test -v ./...
```

---

## API Overview

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/databases` | List all databases and get active database name |
| `POST` | `/api/databases` | Create and switch to a new SQLite database |
| `POST` | `/api/databases/switch` | Switch active database |
| `DELETE` | `/api/databases/{name}` | Delete a database file |
| `POST` | `/api/databases/sample` | Seed active database with sample e-commerce tables |
| `GET` | `/api/schema` | Get schema graph (tables, columns, foreign keys, row counts) |
| `POST` | `/api/tables` | Create a new table with columns and foreign keys |
| `DELETE` | `/api/tables/{name}` | Drop a table |
| `POST` | `/api/tables/link` | Add foreign key relationship between tables |
| `GET` | `/api/data/{table}` | Get paginated table rows with search & sort |
| `POST` | `/api/data/{table}/row` | Insert new row into table |
| `PUT` | `/api/data/{table}/row` | Update existing row by primary key |
| `DELETE` | `/api/data/{table}/row` | Delete row by primary key |
| `GET` | `/api/data/{table}/fk-options` | Fetch readable foreign key options for dropdowns |
| `POST` | `/api/query/preview` | Preview SQL generated from query builder |
| `POST` | `/api/query/build` | Build and execute visual query builder AST |
| `POST` | `/api/query/execute` | Execute arbitrary SQL query |
