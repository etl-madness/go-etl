package flow

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"  // MySQL driver
	_ "github.com/lib/pq"               // PostgreSQL driver
	_ "github.com/microsoft/go-mssqldb" // MSSQL driver
	_ "github.com/sijms/go-ora/v2"      // Pure Go Oracle driver
	_ "modernc.org/sqlite"              // Pure Go SQLite driver
)

type DBHandle struct {
	Conn   *sql.DB
	Driver string
}

type Registry struct {
	dbRegistry  map[string]DBHandle
	dbMu        sync.RWMutex
	varRegistry map[string]interface{}
	varMu       sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		dbRegistry:  make(map[string]DBHandle),
		varRegistry: make(map[string]interface{}),
	}
}

func (r *Registry) SetVar(name string, value interface{}) {
	r.varMu.Lock()
	defer r.varMu.Unlock()
	r.varRegistry[name] = value
}

func (r *Registry) GetVar(name string) interface{} {
	r.varMu.RLock()
	defer r.varMu.RUnlock()
	return r.varRegistry[name]
}

func (r *Registry) GetVarString(name string) string {
	val := r.GetVar(name)
	if val == nil {
		return ""
	}
	if v, ok := val.(string); ok {
		return v
	}
	return fmt.Sprintf("%v", val)
}

func (r *Registry) GetVarInt(name string) int {
	val := r.GetVar(name)
	if val == nil {
		return 0
	}
	if v, ok := val.(int); ok {
		return v
	}
	if str, ok := val.(string); ok {
		if i, err := strconv.Atoi(str); err == nil {
			return i
		}
	}
	return 0
}

func (r *Registry) GetVarBool(name string) bool {
	val := r.GetVar(name)
	if val == nil {
		return false
	}
	if v, ok := val.(bool); ok {
		return v
	}
	if str, ok := val.(string); ok {
		if b, err := strconv.ParseBool(str); err == nil {
			return b
		}
	}
	return false
}

func (r *Registry) GetVarFloat(name string) float64 {
	val := r.GetVar(name)
	if val == nil {
		return 0.0
	}
	if v, ok := val.(float64); ok {
		return v
	}
	if str, ok := val.(string); ok {
		if f, err := r.parseFloat(str); err == nil {
			return f
		}
	}
	return 0.0
}

func (r *Registry) parseFloat(val string) (float64, error) {
	return strconv.ParseFloat(val, 64)
}

func (r *Registry) GetDB(name string) (*sql.DB, error) {
	r.dbMu.RLock()
	defer r.dbMu.RUnlock()

	handle, ok := r.dbRegistry[name]
	if !ok {
		return nil, fmt.Errorf("database connection '%s' not registered", name)
	}
	return handle.Conn, nil
}

func (r *Registry) GetDBHandle(name string) (DBHandle, error) {
	r.dbMu.RLock()
	defer r.dbMu.RUnlock()

	handle, ok := r.dbRegistry[name]
	if !ok {
		return DBHandle{}, fmt.Errorf("database connection '%s' not registered", name)
	}
	return handle, nil
}

func (r *Registry) InitVariables(configs []VariableConfig) error {
	r.varMu.Lock()
	defer r.varMu.Unlock()

	for _, cfg := range configs {
		switch cfg.Type {
		case "int", "integer":
			val, err := strconv.Atoi(cfg.Value)
			if err != nil {
				return fmt.Errorf("invalid int value '%s' for variable '%s'", cfg.Value, cfg.Name)
			}
			r.varRegistry[cfg.Name] = val
		case "bool", "boolean":
			val, err := strconv.ParseBool(cfg.Value)
			if err != nil {
				return fmt.Errorf("invalid bool value '%s' for variable '%s'", cfg.Value, cfg.Name)
			}
			r.varRegistry[cfg.Name] = val
		case "float", "double", "float64":
			val, err := r.parseFloat(cfg.Value)
			if err != nil {
				return fmt.Errorf("invalid float value '%s' for variable '%s'", cfg.Value, cfg.Name)
			}
			r.varRegistry[cfg.Name] = val
		default:
			r.varRegistry[cfg.Name] = cfg.Value
		}
	}
	return nil
}

func (r *Registry) InitDatabases(configs []DatabaseConfig) error {
	r.dbMu.Lock()
	defer r.dbMu.Unlock()

	r.varMu.RLock()
	defer r.varMu.RUnlock()

	for _, cfg := range configs {
		connStr := cfg.ConnectionString

		for name, val := range r.varRegistry {
			placeholder := fmt.Sprintf("{{%s}}", name)
			connStr = strings.ReplaceAll(connStr, placeholder, fmt.Sprintf("%v", val))
		}

		driverName := cfg.Driver
		if driverName == "" {
			driverName = "sqlserver"
		}

		dbConn, err := sql.Open(driverName, connStr)
		if err != nil {
			return fmt.Errorf("failed to open database '%s' (%s): %w", cfg.Name, driverName, err)
		}

		dbConn.SetMaxOpenConns(25)
		dbConn.SetMaxIdleConns(10)
		dbConn.SetConnMaxLifetime(5 * time.Minute)

		r.dbRegistry[cfg.Name] = DBHandle{
			Conn:   dbConn,
			Driver: driverName,
		}
	}
	return nil
}

func (r *Registry) CloseDatabases() {
	r.dbMu.Lock()
	defer r.dbMu.Unlock()

	for name, handle := range r.dbRegistry {
		handle.Conn.Close()
		delete(r.dbRegistry, name)
	}
}

func (r *Registry) CopyVariables() map[string]interface{} {
	r.varMu.RLock()
	defer r.varMu.RUnlock()
	copyMap := make(map[string]interface{})
	for k, v := range r.varRegistry {
		copyMap[k] = v
	}
	return copyMap
}
