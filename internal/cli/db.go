package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/store"
)

func dbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database utilities (schema init/migrate)",
	}
	cmd.AddCommand(dbInitCmd())
	return cmd
}

func dbInitCmd() *cobra.Command {
	var schemaPath string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize schema (sql/schema.sql) in PostgreSQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			if schemaPath == "" {
				schemaPath = "sql/schema.sql"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			if err := applySQLFile(ctx, st, schemaPath); err != nil {
				return fmt.Errorf("apply schema: %w", err)
			}
			migrationFiles, err := filepath.Glob("sql/migrations/*.sql")
			if err != nil {
				return fmt.Errorf("list migrations: %w", err)
			}
			sort.Strings(migrationFiles)
			for _, migration := range migrationFiles {
				if err := applySQLFile(ctx, st, migration); err != nil {
					return fmt.Errorf("apply migration %s: %w", migration, err)
				}
			}
			fmt.Println("ok: schema and migrations applied")
			return nil
		},
	}
	cmd.Flags().StringVar(&schemaPath, "schema", "sql/schema.sql", "Path to schema SQL file")
	return cmd
}

func applySQLFile(ctx context.Context, st *store.Store, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return st.ExecSQL(ctx, string(b))
}
