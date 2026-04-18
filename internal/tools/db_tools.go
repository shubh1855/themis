package tools

import (
	"context"
	"fmt"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/dbx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

func openDB(ctx Context) (*dbx.DB, error) {
	path := models.ArgString(ctx.Req.Args, "database")
	if path == "" {
		path = models.ArgString(ctx.Req.Args, "db")
	}
	if path == "" {
		return nil, fmt.Errorf("missing 'database' argument")
	}
	return dbx.Open(path)
}

// HandleSQLQuery executes a SQL query against a database.
func HandleSQLQuery(ctx Context) models.ToolResponse {
	db, err := openDB(ctx)
	if err != nil {
		return models.ErrorResponsef("sql_query: %v", err)
	}
	defer db.Close()

	query := models.ArgString(ctx.Req.Args, "query")
	if query == "" {
		return models.ErrorResponse("sql_query: missing 'query' argument")
	}

	rows, cols, err := db.Query(context.Background(), query)
	if err != nil {
		return models.ErrorResponsef("sql_query: %v", err)
	}

	return models.SuccessResponse(models.DBResult{
		Columns: cols,
		Rows:    rows,
	})
}

// HandleDBTables lists all tables in the database.
func HandleDBTables(ctx Context) models.ToolResponse {
	db, err := openDB(ctx)
	if err != nil {
		return models.ErrorResponsef("db_tables: %v", err)
	}
	defer db.Close()

	tables, err := db.Tables(context.Background())
	if err != nil {
		return models.ErrorResponsef("db_tables: %v", err)
	}
	return models.SuccessResponse(tables)
}

// HandleDBSchema returns the schema for a table.
func HandleDBSchema(ctx Context) models.ToolResponse {
	db, err := openDB(ctx)
	if err != nil {
		return models.ErrorResponsef("db_schema: %v", err)
	}
	defer db.Close()

	table := models.ArgString(ctx.Req.Args, "table")
	if table == "" {
		return models.ErrorResponse("db_schema: missing 'table' argument")
	}

	schema, err := db.Schema(context.Background(), table)
	if err != nil {
		return models.ErrorResponsef("db_schema: %v", err)
	}
	return models.SuccessResponse(schema)
}

// HandleDBMigrate runs a SQL migration script.
func HandleDBMigrate(ctx Context) models.ToolResponse {
	db, err := openDB(ctx)
	if err != nil {
		return models.ErrorResponsef("db_migrate: %v", err)
	}
	defer db.Close()

	sql := models.ArgString(ctx.Req.Args, "sql")
	file := models.ArgString(ctx.Req.Args, "file")

	if sql != "" {
		if err := db.Migrate(context.Background(), sql); err != nil {
			return models.ErrorResponsef("db_migrate: %v", err)
		}
		return models.SuccessResponse("migration applied")
	}

	if file != "" {
		if err := db.MigrateFile(context.Background(), file); err != nil {
			return models.ErrorResponsef("db_migrate: %v", err)
		}
		return models.SuccessResponse("migration file applied: " + file)
	}

	return models.ErrorResponse("db_migrate: provide 'sql' or 'file' argument")
}
