package database

import (
	"database/sql"
	"fmt"
)

// LoadSampleEcommerceDB sets up a complete e-commerce relational schema with sample records
func LoadSampleEcommerceDB(db *sql.DB) error {
	schemaQueries := []string{
		`DROP TABLE IF EXISTS "order_items";`,
		`DROP TABLE IF EXISTS "orders";`,
		`DROP TABLE IF EXISTS "products";`,
		`DROP TABLE IF EXISTS "customers";`,

		`CREATE TABLE "customers" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"name" TEXT NOT NULL,
			"email" TEXT UNIQUE NOT NULL,
			"phone" TEXT,
			"city" TEXT,
			"country" TEXT DEFAULT 'USA',
			"created_at" DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE "products" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"name" TEXT NOT NULL,
			"sku" TEXT UNIQUE,
			"category" TEXT,
			"price" REAL NOT NULL,
			"stock_qty" INTEGER DEFAULT 0,
			"is_active" BOOLEAN DEFAULT 1
		);`,

		`CREATE TABLE "orders" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"customer_id" INTEGER NOT NULL,
			"order_date" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"status" TEXT DEFAULT 'PENDING',
			"shipping_address" TEXT,
			"total_amount" REAL DEFAULT 0.00,
			FOREIGN KEY ("customer_id") REFERENCES "customers" ("id") ON DELETE CASCADE
		);`,

		`CREATE TABLE "order_items" (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"order_id" INTEGER NOT NULL,
			"product_id" INTEGER NOT NULL,
			"quantity" INTEGER DEFAULT 1,
			"unit_price" REAL NOT NULL,
			FOREIGN KEY ("order_id") REFERENCES "orders" ("id") ON DELETE CASCADE,
			FOREIGN KEY ("product_id") REFERENCES "products" ("id") ON DELETE RESTRICT
		);`,

		// Sample Customers
		`INSERT INTO "customers" ("name", "email", "phone", "city", "country") VALUES
			('Alice Johnson', 'alice@example.com', '+1-555-0101', 'San Francisco', 'USA'),
			('Bob Smith', 'bob.smith@techcorp.io', '+1-555-0102', 'Seattle', 'USA'),
			('Clara Rossi', 'clara.rossi@milanodesign.it', '+39-02-55512', 'Milan', 'Italy'),
			('David Kim', 'dkim@seoulvibe.kr', '+82-2-555-9876', 'Seoul', 'South Korea'),
			('Elena Rostova', 'elena@novatech.org', '+49-30-55533', 'Berlin', 'Germany');`,

		// Sample Products
		`INSERT INTO "products" ("name", "sku", "category", "price", "stock_qty", "is_active") VALUES
			('Mechanical Keyboard RGB', 'TECH-KEY-001', 'Electronics', 129.99, 45, 1),
			('Wireless Ergonomic Mouse', 'TECH-MOU-002', 'Electronics', 59.95, 120, 1),
			('4K Ultra-HD Monitor 27"', 'TECH-MON-003', 'Displays', 389.00, 18, 1),
			('Noise-Cancelling Headphones', 'TECH-AUD-004', 'Audio', 199.50, 60, 1),
			('Solid Oak Standing Desk', 'FURN-DSK-101', 'Furniture', 549.00, 12, 1),
			('USB-C Aluminum Hub 7-in-1', 'TECH-ACC-202', 'Accessories', 39.99, 85, 1);`,

		// Sample Orders
		`INSERT INTO "orders" ("customer_id", "order_date", "status", "shipping_address", "total_amount") VALUES
			(1, '2026-09-15 10:30:00', 'DELIVERED', '742 Evergreen Terrace, San Francisco, CA', 189.94),
			(2, '2026-09-18 14:15:00', 'SHIPPED', '100 Pike Place, Seattle, WA', 588.50),
			(3, '2026-09-20 09:00:00', 'PROCESSING', 'Via Montenapoleone 8, Milan', 129.99),
			(1, '2026-09-21 16:45:00', 'PENDING', '742 Evergreen Terrace, San Francisco, CA', 549.00),
			(4, '2026-09-22 11:20:00', 'PROCESSING', 'Teheran-ro 152, Gangnam, Seoul', 239.49);`,

		// Sample Order Items
		`INSERT INTO "order_items" ("order_id", "product_id", "quantity", "unit_price") VALUES
			(1, 1, 1, 129.99),
			(1, 2, 1, 59.95),
			(2, 3, 1, 389.00),
			(2, 4, 1, 199.50),
			(3, 1, 1, 129.99),
			(4, 5, 1, 549.00),
			(5, 4, 1, 199.50),
			(5, 6, 1, 39.99);`,
	}

	for _, q := range schemaQueries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("error executing sample migration: %w\nQuery: %s", err, q)
		}
	}

	return nil
}
