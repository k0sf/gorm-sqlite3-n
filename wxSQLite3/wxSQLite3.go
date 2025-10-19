package wxSQLite3

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/k0sf/gorm-sqlite3-n/go-wxsqlite3"
	sqlite3 "github.com/k0sf/gorm-sqlite3-n/go-wxsqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

var (
	// ErrConstraintsNotImplemented indicates that constraint operations are not supported
	ErrConstraintsNotImplemented = errors.New("constraints not implemented on sqlite, consider using DisableForeignKeyConstraintWhenMigrating, more details https://github.com/go-gorm/gorm/wiki/GORM-V2-Release-Note-Draft#all-new-migrator")
)

const (
	// DriverName is the default driver name for SQLite
	DriverName = "sqlite3"
)

// Dialector implements gorm.Dialector for SQLite with wxsqlite3 encryption support
type Dialector struct {
	DriverName string
	DSN        string
	Conn       gorm.ConnPool
}

// ResetDBKey resets the database encryption password.
// It opens the database with the old key and changes it to the new key.
// The database uses AES128 encryption by default (wxsqlite3).
func ResetDBKey(dbName string, oldKey string, newKey string) error {
	return ResetDBKeyWithContext(context.Background(), dbName, oldKey, newKey)
}

// ResetDBKeyWithContext resets the database encryption password with context support.
// It opens the database with the old key and changes it to the new key.
// The operation can be cancelled via the provided context.
func ResetDBKeyWithContext(ctx context.Context, dbName string, oldKey string, newKey string) error {
	sr := sqlite3.Cpp{}

	if err := sr.Open(dbName, oldKey); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		sr.Close()
	}()

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := sr.ReKey(newKey); err != nil {
		return fmt.Errorf("failed to rekey database: %w", err)
	}

	return nil
}

// Open creates a new SQLite dialector with the given DSN.
// The DSN can include encryption parameters for wxsqlite3.
//
// Example: "test.db?_key=mypassword"
func Open(dsn string) gorm.Dialector {
	return &Dialector{DSN: dsn}
}

// Name returns the name of the dialector
func (dialector Dialector) Name() string {
	return "sqlite"
}

// Initialize initializes the dialector with the given GORM DB instance
func (dialector Dialector) Initialize(db *gorm.DB) (err error) {
	if dialector.DriverName == "" {
		dialector.DriverName = DriverName
	}

	// Register callbacks
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
		LastInsertIDReversed: true,
	})

	// Initialize connection pool
	if dialector.Conn != nil {
		db.ConnPool = dialector.Conn
	} else {
		db.ConnPool, err = sql.Open(dialector.DriverName, dialector.DSN)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		// Test the connection
		if sqlDB, ok := db.ConnPool.(*sql.DB); ok {
			if err = sqlDB.Ping(); err != nil {
				return fmt.Errorf("failed to ping database: %w", err)
			}
		}
	}

	// Register clause builders
	for k, v := range dialector.ClauseBuilders() {
		db.ClauseBuilders[k] = v
	}

	return nil
}

// ClauseBuilders returns custom clause builders for SQLite
func (dialector Dialector) ClauseBuilders() map[string]clause.ClauseBuilder {
	return map[string]clause.ClauseBuilder{
		"INSERT": func(c clause.Clause, builder clause.Builder) {
			if insert, ok := c.Expression.(clause.Insert); ok {
				if stmt, ok := builder.(*gorm.Statement); ok {
					stmt.WriteString("INSERT ")
					if insert.Modifier != "" {
						stmt.WriteString(insert.Modifier)
						stmt.WriteByte(' ')
					}

					stmt.WriteString("INTO ")
					if insert.Table.Name == "" {
						stmt.WriteQuoted(stmt.Table)
					} else {
						stmt.WriteQuoted(insert.Table)
					}
					return
				}
			}

			c.Build(builder)
		},
		"LIMIT": func(c clause.Clause, builder clause.Builder) {
			if limit, ok := c.Expression.(clause.Limit); ok {
				var lmt = -1
				if limit.Limit != nil && *limit.Limit >= 0 {
					lmt = *limit.Limit
				}
				if lmt >= 0 || limit.Offset > 0 {
					builder.WriteString("LIMIT ")
					builder.WriteString(strconv.Itoa(lmt))
				}
				if limit.Offset > 0 {
					builder.WriteString(" OFFSET ")
					builder.WriteString(strconv.Itoa(limit.Offset))
				}
			}
		},
		"FOR": func(c clause.Clause, builder clause.Builder) {
			if _, ok := c.Expression.(clause.Locking); ok {
				// SQLite3 does not support row-level locking
				return
			}
			c.Build(builder)
		},
	}
}

// DefaultValueOf returns the default value clause for a field
func (dialector Dialector) DefaultValueOf(field *schema.Field) clause.Expression {
	if field.AutoIncrement {
		return clause.Expr{SQL: "NULL"}
	}

	// doesn't work, will raise error
	return clause.Expr{SQL: "DEFAULT"}
}

// Migrator returns the migrator instance for this dialector
func (dialector Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return Migrator{migrator.Migrator{Config: migrator.Config{
		DB:                          db,
		Dialector:                   dialector,
		CreateIndexAfterCreateTable: true,
	}}}
}

// BindVarTo writes the bind variable placeholder to the writer
func (dialector Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}

// QuoteTo writes the quoted identifier to the writer
func (dialector Dialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('`')
	if strings.Contains(str, ".") {
		for idx, str := range strings.Split(str, ".") {
			if idx > 0 {
				writer.WriteString(".`")
			}
			writer.WriteString(str)
			writer.WriteByte('`')
		}
	} else {
		writer.WriteString(str)
		writer.WriteByte('`')
	}
}

// Explain returns the explained SQL statement
func (dialector Dialector) Explain(sql string, vars ...interface{}) string {
	return logger.ExplainSQL(sql, nil, `"`, vars...)
}

// DataTypeOf returns the data type string for a field
func (dialector Dialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "numeric"
	case schema.Int, schema.Uint:
		if field.AutoIncrement && !field.PrimaryKey {
			// https://www.sqlite.org/autoinc.html
			return "integer PRIMARY KEY AUTOINCREMENT"
		}
		return "integer"
	case schema.Float:
		return "real"
	case schema.String:
		return "text"
	case schema.Time:
		return "datetime"
	case schema.Bytes:
		return "blob"
	}

	return string(field.DataType)
}

// SavePoint creates a savepoint with the given name
func (dialector Dialector) SavePoint(tx *gorm.DB, name string) error {
	return tx.Exec("SAVEPOINT " + name).Error
}

// RollbackTo rolls back to the savepoint with the given name
func (dialector Dialector) RollbackTo(tx *gorm.DB, name string) error {
	return tx.Exec("ROLLBACK TO SAVEPOINT " + name).Error
}

// ========================================================================================
// ======================    Migrator   ===================================================
// ========================================================================================

// Migrator implements gorm.Migrator for SQLite
type Migrator struct {
	migrator.Migrator
}

// RunWithoutForeignKey runs the given function with foreign key checks disabled
func (m *Migrator) RunWithoutForeignKey(fc func() error) error {
	var enabled int
	if err := m.DB.Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil {
		return fmt.Errorf("failed to check foreign key status: %w", err)
	}

	if enabled == 1 {
		if err := m.DB.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return fmt.Errorf("failed to disable foreign keys: %w", err)
		}
		defer func() {
			if err := m.DB.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
				log.Printf("warning: failed to re-enable foreign keys: %v", err)
			}
		}()
	}

	return fc()
}

// HasTable checks if a table exists
func (m Migrator) HasTable(value interface{}) bool {
	var count int
	m.Migrator.RunWithValue(value, func(stmt *gorm.Statement) error {
		return m.DB.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", stmt.Table).Row().Scan(&count)
	})
	return count > 0
}

// DropTable drops the specified tables
func (m Migrator) DropTable(values ...interface{}) error {
	return m.RunWithoutForeignKey(func() error {
		values = m.ReorderModels(values, false)
		tx := m.DB.Session(&gorm.Session{})

		for i := len(values) - 1; i >= 0; i-- {
			if err := m.RunWithValue(values[i], func(stmt *gorm.Statement) error {
				return tx.Exec("DROP TABLE IF EXISTS ?", clause.Table{Name: stmt.Table}).Error
			}); err != nil {
				return fmt.Errorf("failed to drop table: %w", err)
			}
		}

		return nil
	})
}

// HasColumn checks if a column exists in a table
func (m Migrator) HasColumn(value interface{}, name string) bool {
	var count int
	m.Migrator.RunWithValue(value, func(stmt *gorm.Statement) error {
		if field := stmt.Schema.LookUpField(name); field != nil {
			name = field.DBName
		}

		if name != "" {
			m.DB.Raw(
				"SELECT count(*) FROM sqlite_master WHERE type = ? AND tbl_name = ? AND (sql LIKE ? OR sql LIKE ? OR sql LIKE ?)",
				"table", stmt.Table, `%"`+name+`" %`, `%`+name+` %`, "%`"+name+"`%",
			).Row().Scan(&count)
		}
		return nil
	})
	return count > 0
}

// AlterColumn alters a column in a table
func (m Migrator) AlterColumn(value interface{}, name string) error {
	return m.RunWithoutForeignKey(func() error {
		return m.RunWithValue(value, func(stmt *gorm.Statement) error {
			field := stmt.Schema.LookUpField(name)
			if field == nil {
				return fmt.Errorf("field %q not found", name)
			}

			var createSQL string
			newTableName := stmt.Table + "__temp"

			if err := m.DB.Raw("SELECT sql FROM sqlite_master WHERE type = ? AND tbl_name = ? AND name = ?",
				"table", stmt.Table, stmt.Table).Row().Scan(&createSQL); err != nil {
				return fmt.Errorf("failed to get table schema: %w", err)
			}

			// Build regex pattern with proper escaping
			fieldPattern := fmt.Sprintf("(`|'|\"| )%s(`|'|\"| ) .*?,", regexp.QuoteMeta(field.DBName))
			fieldReg, err := regexp.Compile(fieldPattern)
			if err != nil {
				return fmt.Errorf("failed to compile field regex: %w", err)
			}

			tablePattern := fmt.Sprintf(" ('|`|\"| )%s('|`|\"| ) ", regexp.QuoteMeta(stmt.Table))
			tableReg, err := regexp.Compile(tablePattern)
			if err != nil {
				return fmt.Errorf("failed to compile table regex: %w", err)
			}

			// Get the full data type definition
			dataType := m.FullDataTypeOf(field).SQL

			// Replace table name and field definition with actual data type (not placeholder)
			createSQL = tableReg.ReplaceAllString(createSQL, fmt.Sprintf(" `%s` ", newTableName))
			createSQL = fieldReg.ReplaceAllString(createSQL, fmt.Sprintf("`%s` %s,", field.DBName, dataType))

			// Get all columns
			var columns []string
			columnTypes, err := m.DB.Migrator().ColumnTypes(value)
			if err != nil {
				return fmt.Errorf("failed to get column types: %w", err)
			}

			for _, columnType := range columnTypes {
				columns = append(columns, fmt.Sprintf("`%s`", columnType.Name()))
			}

			// Execute migration in transaction
			return m.DB.Transaction(func(tx *gorm.DB) error {
				queries := []string{
					createSQL,
					fmt.Sprintf("INSERT INTO `%s`(%s) SELECT %s FROM `%s`",
						newTableName, strings.Join(columns, ","), strings.Join(columns, ","), stmt.Table),
					fmt.Sprintf("DROP TABLE `%s`", stmt.Table),
					fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", newTableName, stmt.Table),
				}

				for _, query := range queries {
					if err := tx.Exec(query).Error; err != nil {
						return fmt.Errorf("failed to execute query %q: %w", query, err)
					}
				}

				return nil
			})
		})
	})
}

// DropColumn drops a column from a table
func (m Migrator) DropColumn(value interface{}, name string) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if field := stmt.Schema.LookUpField(name); field != nil {
			name = field.DBName
		}

		var createSQL string
		newTableName := stmt.Table + "__temp"

		if err := m.DB.Raw("SELECT sql FROM sqlite_master WHERE type = ? AND tbl_name = ? AND name = ?",
			"table", stmt.Table, stmt.Table).Row().Scan(&createSQL); err != nil {
			return fmt.Errorf("failed to get table schema: %w", err)
		}

		// Build regex patterns
		fieldPattern := fmt.Sprintf("(`|'|\"| )%s(`|'|\"| ) .*?,", regexp.QuoteMeta(name))
		fieldReg, err := regexp.Compile(fieldPattern)
		if err != nil {
			return fmt.Errorf("failed to compile field regex: %w", err)
		}

		tablePattern := fmt.Sprintf(" ('|`|\"| )%s('|`|\"| ) ", regexp.QuoteMeta(stmt.Table))
		tableReg, err := regexp.Compile(tablePattern)
		if err != nil {
			return fmt.Errorf("failed to compile table regex: %w", err)
		}

		// Replace table name and remove field
		createSQL = tableReg.ReplaceAllString(createSQL, fmt.Sprintf(" `%s` ", newTableName))
		createSQL = fieldReg.ReplaceAllString(createSQL, "")

		// Get columns excluding the dropped one
		var columns []string
		columnTypes, err := m.DB.Migrator().ColumnTypes(value)
		if err != nil {
			return fmt.Errorf("failed to get column types: %w", err)
		}

		for _, columnType := range columnTypes {
			if columnType.Name() != name {
				columns = append(columns, fmt.Sprintf("`%s`", columnType.Name()))
			}
		}

		// Execute migration in transaction
		return m.DB.Transaction(func(tx *gorm.DB) error {
			queries := []string{
				createSQL,
				fmt.Sprintf("INSERT INTO `%s`(%s) SELECT %s FROM `%s`",
					newTableName, strings.Join(columns, ","), strings.Join(columns, ","), stmt.Table),
				fmt.Sprintf("DROP TABLE `%s`", stmt.Table),
				fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", newTableName, stmt.Table),
			}

			for _, query := range queries {
				if err := tx.Exec(query).Error; err != nil {
					return fmt.Errorf("failed to execute query %q: %w", query, err)
				}
			}

			return nil
		})
	})
}

// CreateConstraint creates a constraint (not implemented for SQLite)
func (m Migrator) CreateConstraint(interface{}, string) error {
	return ErrConstraintsNotImplemented
}

// DropConstraint drops a constraint (not implemented for SQLite)
func (m Migrator) DropConstraint(interface{}, string) error {
	return ErrConstraintsNotImplemented
}

// HasConstraint checks if a constraint exists
func (m Migrator) HasConstraint(value interface{}, name string) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		m.DB.Raw(
			"SELECT count(*) FROM sqlite_master WHERE type = ? AND tbl_name = ? AND (sql LIKE ? OR sql LIKE ? OR sql LIKE ?)",
			"table", stmt.Table, `%CONSTRAINT "`+name+`" %`, `%CONSTRAINT `+name+` %`, "%CONSTRAINT `"+name+"`%",
		).Row().Scan(&count)
		return nil
	})

	return count > 0
}

// CurrentDatabase returns the current database name
func (m Migrator) CurrentDatabase() (name string) {
	var null interface{}
	m.DB.Raw("PRAGMA database_list").Row().Scan(&null, &name, &null)
	return
}

// BuildIndexOptions builds index options from schema index options
func (m Migrator) BuildIndexOptions(opts []schema.IndexOption, stmt *gorm.Statement) (results []interface{}) {
	for _, opt := range opts {
		str := stmt.Quote(opt.DBName)
		if opt.Expression != "" {
			str = opt.Expression
		}

		if opt.Collate != "" {
			str += " COLLATE " + opt.Collate
		}

		if opt.Sort != "" {
			str += " " + opt.Sort
		}
		results = append(results, clause.Expr{SQL: str})
	}
	return
}

// CreateIndex creates an index
func (m Migrator) CreateIndex(value interface{}, name string) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		idx := stmt.Schema.LookIndex(name)
		if idx == nil {
			return fmt.Errorf("index %q not found", name)
		}

		opts := m.BuildIndexOptions(idx.Fields, stmt)
		values := []interface{}{clause.Column{Name: idx.Name}, clause.Table{Name: stmt.Table}, opts}

		createIndexSQL := "CREATE "
		if idx.Class != "" {
			createIndexSQL += idx.Class + " "
		}
		createIndexSQL += "INDEX ?"

		if idx.Type != "" {
			createIndexSQL += " USING " + idx.Type
		}
		createIndexSQL += " ON ??"

		if idx.Where != "" {
			createIndexSQL += " WHERE " + idx.Where
		}

		return m.DB.Exec(createIndexSQL, values...).Error
	})
}

// HasIndex checks if an index exists
func (m Migrator) HasIndex(value interface{}, name string) bool {
	var count int
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if idx := stmt.Schema.LookIndex(name); idx != nil {
			name = idx.Name
		}

		if name != "" {
			m.DB.Raw(
				"SELECT count(*) FROM sqlite_master WHERE type = ? AND tbl_name = ? AND name = ?",
				"index", stmt.Table, name,
			).Row().Scan(&count)
		}
		return nil
	})
	return count > 0
}

// RenameIndex renames an index
func (m Migrator) RenameIndex(value interface{}, oldName, newName string) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		var sql string
		if err := m.DB.Raw("SELECT sql FROM sqlite_master WHERE type = ? AND tbl_name = ? AND name = ?",
			"index", stmt.Table, oldName).Row().Scan(&sql); err != nil {
			return fmt.Errorf("failed to find index %q: %w", oldName, err)
		}

		if sql == "" {
			return fmt.Errorf("index %q not found", oldName)
		}

		return m.DB.Exec(strings.Replace(sql, oldName, newName, 1)).Error
	})
}

// DropIndex drops an index
func (m Migrator) DropIndex(value interface{}, name string) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if idx := stmt.Schema.LookIndex(name); idx != nil {
			name = idx.Name
		}

		return m.DB.Exec("DROP INDEX ?", clause.Column{Name: name}).Error
	})
}
