package codegen

import (
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
)

func objectUsesSQLEnum(object ObjectView, enum BackendEnumView) bool {
	for _, field := range object.Fields {
		if field.Enum == enum.Name {
			return true
		}
	}
	return false
}

func enumMigrationVersion(filename string) (*big.Int, error) {
	prefix, _, ok := strings.Cut(filepath.Base(filename), "_")
	value, valid := new(big.Int).SetString(prefix, 10)
	if !ok || !valid || value.Sign() < 0 {
		return nil, fmt.Errorf("migration %s must have a numeric version prefix", filename)
	}
	return value, nil
}

// A shared enum migration must precede each generated table that uses it on a
// clean replay. Refuse to append a type after an existing dependent CREATE TABLE.
// The generator cannot infer which old migrations have already been applied.
func validateBackendEnumMigrationOrder(cfg Config, objects []ObjectView, enums []BackendEnumView) error {
	renderer := NewRenderer(cfg)
	for _, enum := range enums {
		if !enum.MigrationEnabled {
			continue
		}
		pair, exists, err := renderer.findMigrationPair(enum.MigrationName)
		if err != nil {
			return err
		}
		for _, object := range objects {
			if object.Migration.Enabled && object.Migration.Name == enum.MigrationName {
				return fmt.Errorf("enum %q migration name %q conflicts with table migration for %s", enum.Name, enum.MigrationName, object.Name)
			}
			if !object.Migration.Enabled || !objectUsesSQLEnum(object, enum) {
				continue
			}
			table, tableExists, err := renderer.findMigrationPair(object.Migration.Name)
			if err != nil {
				return err
			}
			if !tableExists {
				continue
			}
			if !exists {
				return fmt.Errorf("enum %q: cannot append initial enum migration %q after existing dependent table migration %s; keep enum migration.enabled: false and provide a reviewed forward migration for an existing database, or arrange an earlier enum migration explicitly for a new unapplied history", enum.Name, enum.MigrationName, table.UpPath)
			}
			enumVersion, err := enumMigrationVersion(pair.UpPath)
			if err != nil {
				return err
			}
			tableVersion, err := enumMigrationVersion(table.UpPath)
			if err != nil {
				return err
			}
			if enumVersion.Cmp(tableVersion) >= 0 {
				return fmt.Errorf("enum %q migration %s must precede dependent table migration %s; do not reorder already applied migrations", enum.Name, pair.UpPath, table.UpPath)
			}
		}
	}
	return nil
}

func (r Renderer) RenderBackendEnumMigrations(objects []ObjectView) error {
	enums, err := collectBackendEnums(objects)
	if err != nil {
		return err
	}
	if err := validateBackendEnumMigrationOrder(r.Config, objects, enums); err != nil {
		return err
	}
	for _, enum := range enums {
		if !enum.MigrationEnabled {
			continue
		}
		// Enum migrations are immutable once created, including check mode.
		// Schema changes update Go/TS output, not historical CREATE TYPE SQL.
		immutable := r
		immutable.Config.MigrationsOverwrite = false
		if err := immutable.renderScaffoldMigration(
			enum.MigrationName,
			filepath.Join(r.Config.TemplateDir, "sql", "enum.up.sql.tmpl"),
			filepath.Join(r.Config.TemplateDir, "sql", "enum.down.sql.tmpl"),
			enum,
		); err != nil {
			return err
		}
	}
	return nil
}
