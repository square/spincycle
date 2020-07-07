package migrations_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	testdb "github.com/square/spincycle/v2/request-manager/test/db"

	"github.com/go-test/deep"
)

// Compare the result of applying all the SQL migrations to a database to
// the result of simply loading in the schema file, to make sure the
// resulting schemas are identical.
func TestMigrationsEqualSchema(t *testing.T) {
	SchemaFile, err := filepath.Abs("../request_manager_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	MigrationPath, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	dbm, err := testdb.NewManager()

	// Create a db and load in the request manager schema file.
	schemaDBName, err := dbm.CreateBlank()
	if err != nil {
		t.Fatal(err)
	}
	defer dbm.Destroy(schemaDBName)

	err = dbm.LoadSQLFile(schemaDBName, SchemaFile)
	if err != nil {
		t.Fatal(err)
	}

	schemaDB, err := dbm.Connect(schemaDBName)

	// Create a db and load in all the migration SQL files.
	migrationDBName, err := dbm.CreateBlank()
	if err != nil {
		t.Fatal(err)
	}
	defer dbm.Destroy(migrationDBName)

	err = filepath.Walk(MigrationPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory: %s", err)
		}

		// If it's not a SQL file, don't load it.
		if info.IsDir() || !strings.Contains(info.Name(), ".sql") {
			return nil
		}

		err = dbm.LoadSQLFile(migrationDBName, path)
		if err != nil {
			return fmt.Errorf("error loading SQL from file %s: %s", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	migrationDB, err := dbm.Connect(migrationDBName)

	// 1. Compare the dbs created from the schema file and migration files to
	// make sure the tables are all identical.
	showTablesQuery := "SELECT table_name FROM information_schema.tables where table_schema = ? ORDER BY table_name"

	// Get all the tables in the db created from the schema.
	rows, err := schemaDB.Query(showTablesQuery, schemaDBName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var tablesFromSchema []string
	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			t.Fatal(err)
		}

		tablesFromSchema = append(tablesFromSchema, table)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}

	// Get all the tables in the db created from the migrations.
	rows, err = migrationDB.Query(showTablesQuery, migrationDBName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var tablesFromMigrations []string
	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			t.Fatal(err)
		}

		tablesFromMigrations = append(tablesFromMigrations, table)
	}
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}

	if diff := deep.Equal(tablesFromSchema, tablesFromMigrations); diff != nil {
		t.Fatalf("Tables present differ between schemas: %v", diff)
	}

	// 2. Compare CREATE TABLE statements for each table.
	for _, table := range tablesFromSchema {
		query := fmt.Sprintf("SHOW CREATE TABLE %s", table)

		var tableName string
		var schemaCreate string
		err := schemaDB.QueryRow(query).Scan(&tableName, &schemaCreate)
		if err != nil {
			t.Fatal(err)
		}

		var migrationCreate string
		err = migrationDB.QueryRow(query).Scan(&tableName, &migrationCreate)
		if err != nil {
			t.Fatal(err)
		}

		if schemaCreate != migrationCreate {
			t.Errorf("CREATE TABLE for table %q differs between schemas. From single schema file:\n%s\nFrom migration files:\n%s", table, schemaCreate, migrationCreate)
		}
	}
}
