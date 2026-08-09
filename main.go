package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	// Database Drivers
	_ "github.com/go-sql-driver/mysql"  // MySQL driver
	_ "github.com/lib/pq"               // PostgreSQL driver
	_ "github.com/microsoft/go-mssqldb" // MSSQL driver
	_ "github.com/sijms/go-ora/v2"      // Pure Go Oracle driver
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

type VariableConfig struct {
	Name  string
	Type  string
	Value string
}

type DatabaseConfig struct {
	Name             string
	Driver           string
	ConnectionString string
}

type ScriptItem struct {
	ID          string
	Language    string
	DBName      string
	TargetDB    string
	TargetTable string
	BatchSize   int
	VarName     string
	OutputVar   string
	Code        string
}

type NodeKind int

const (
	NodeScript NodeKind = iota
	NodeGroup
	NodeIf
	NodeForEach
	NodeParallel
)

type PipelineNode struct {
	Kind          NodeKind
	MaxThreads    int
	Script        *ScriptItem
	GroupID       string
	IfVar         string
	IfEquals      string
	ForEachScript *ScriptItem
	Children      []PipelineNode
	ElseNodes     []PipelineNode
}

type ScriptResult struct {
	ScriptID      string `json:"script_id"`
	ReturnCode    any    `json:"return_code"`
	ResultsString string `json:"results_string"`
}

type DBHandle struct {
	Conn   *sql.DB
	Driver string
}

var (
	dbRegistry  = make(map[string]DBHandle)
	dbMu        sync.RWMutex
	varRegistry = make(map[string]interface{})
	varMu       sync.RWMutex
	resultsMu   sync.Mutex
)

// -----------------------------------------------------------------------------
// XSD VALIDATION STEP
// -----------------------------------------------------------------------------

func validateXSD(xmlPath string, xsdPath string) error {
	if _, err := os.Stat(xsdPath); os.IsNotExist(err) {
		return fmt.Errorf("XSD schema file not found at path: %s", xsdPath)
	}

	if _, err := exec.LookPath("xmllint"); err != nil {
		return fmt.Errorf("'xmllint' executable not found in PATH. Please install libxml2-utils / xmllint to enable XSD validation")
	}

	cmd := exec.Command("xmllint", "--schema", xsdPath, "--noout", xmlPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		return fmt.Errorf("XSD Schema Validation Failure:\n%s", errStr)
	}

	return nil
}

// -----------------------------------------------------------------------------
// VARIABLE & DATABASE REGISTRATION
// -----------------------------------------------------------------------------

func GetVar(name string) interface{} {
	varMu.RLock()
	defer varMu.RUnlock()
	return varRegistry[name]
}

func GetVarString(name string) string {
	if v, ok := GetVar(name).(string); ok {
		return v
	}
	return fmt.Sprintf("%v", GetVar(name))
}

func GetVarInt(name string) int {
	if v, ok := GetVar(name).(int); ok {
		return v
	}
	if str, ok := GetVar(name).(string); ok {
		if i, err := strconv.Atoi(str); err == nil {
			return i
		}
	}
	return 0
}

func GetVarBool(name string) bool {
	if v, ok := GetVar(name).(bool); ok {
		return v
	}
	return false
}

func GetVarFloat(name string) float64 {
	if v, ok := GetVar(name).(float64); ok {
		return v
	}
	return 0.0
}

func GetDB(name string) (*sql.DB, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	handle, ok := dbRegistry[name]
	if !ok {
		return nil, fmt.Errorf("database connection '%s' not registered", name)
	}
	return handle.Conn, nil
}

func GetDBHandle(name string) (DBHandle, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	handle, ok := dbRegistry[name]
	if !ok {
		return DBHandle{}, fmt.Errorf("database connection '%s' not registered", name)
	}
	return handle, nil
}

// formatPlaceholder formats query placeholders based on the destination database driver syntax
func formatPlaceholder(driver string, paramIdx int) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pgx", "pq":
		return fmt.Sprintf("$%d", paramIdx)
	case "mysql", "sqlite", "sqlite3":
		return "?"
	case "oracle", "godror", "go-ora":
		return fmt.Sprintf(":%d", paramIdx)
	case "sqlserver", "mssql":
		fallthrough
	default:
		return fmt.Sprintf("@p%d", paramIdx)
	}
}

func StreamETL(srcDBName, queryStr, dstDBName, targetTable string, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 500
	}

	varMu.RLock()
	for name, val := range varRegistry {
		placeholder := fmt.Sprintf("{{%s}}", name)
		queryStr = strings.ReplaceAll(queryStr, placeholder, fmt.Sprintf("%v", val))
	}
	varMu.RUnlock()

	srcHandle, err := GetDBHandle(srcDBName)
	if err != nil {
		return 0, fmt.Errorf("source db error: %w", err)
	}
	srcDB := srcHandle.Conn

	dstHandle, err := GetDBHandle(dstDBName)
	if err != nil {
		return 0, fmt.Errorf("destination db error: %w", err)
	}
	dstDB := dstHandle.Conn
	dstDriver := dstHandle.Driver

	rows, err := srcDB.Query(queryStr)
	if err != nil {
		return 0, fmt.Errorf("source query error: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve columns: %w", err)
	}

	if len(cols) == 0 {
		return 0, fmt.Errorf("query returned 0 columns")
	}

	colList := strings.Join(cols, ", ")
	var totalInserted int64 = 0
	var batchRows [][]interface{}

	flushBatch := func() error {
		if len(batchRows) == 0 {
			return nil
		}

		var valuePlaceholders []string
		var valueArgs []interface{}
		paramIdx := 1

		for _, row := range batchRows {
			var placeholders []string
			for _, val := range row {
				placeholders = append(placeholders, formatPlaceholder(dstDriver, paramIdx))
				valueArgs = append(valueArgs, val)
				paramIdx++
			}
			valuePlaceholders = append(valuePlaceholders, "("+strings.Join(placeholders, ", ")+")")
		}

		insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", targetTable, colList, strings.Join(valuePlaceholders, ", "))
		_, err := dstDB.Exec(insertSQL, valueArgs...)
		if err != nil {
			return fmt.Errorf("batch insert error on table '%s': %w", targetTable, err)
		}

		totalInserted += int64(len(batchRows))
		batchRows = batchRows[:0]
		return nil
	}

	for rows.Next() {
		scans := make([]interface{}, len(cols))
		vals := make([]interface{}, len(cols))
		for i := range scans {
			scans[i] = &vals[i]
		}

		if err := rows.Scan(scans...); err != nil {
			return totalInserted, fmt.Errorf("row scan error: %w", err)
		}

		rowCopy := make([]interface{}, len(cols))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				rowCopy[i] = string(b)
			} else {
				rowCopy[i] = v
			}
		}

		batchRows = append(batchRows, rowCopy)

		if len(batchRows) >= batchSize {
			if err := flushBatch(); err != nil {
				return totalInserted, err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return totalInserted, fmt.Errorf("rows iteration error: %w", err)
	}

	if err := flushBatch(); err != nil {
		return totalInserted, err
	}

	return totalInserted, nil
}

// -----------------------------------------------------------------------------
// XML PARSING
// -----------------------------------------------------------------------------

func parseXMLConfig(xmlData []byte) ([]VariableConfig, []DatabaseConfig, []PipelineNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var vars []VariableConfig
	var dbs []DatabaseConfig
	var nodes []PipelineNode
	scriptIndex := 1

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}

		if se, ok := tok.(xml.StartElement); ok {
			elemName := strings.ToLower(se.Name.Local)
			if elemName == "variable" {
				var vCfg VariableConfig
				for _, attr := range se.Attr {
					if strings.EqualFold(attr.Name.Local, "name") {
						vCfg.Name = attr.Value
					} else if strings.EqualFold(attr.Name.Local, "type") {
						vCfg.Type = strings.ToLower(attr.Value)
					} else if strings.EqualFold(attr.Name.Local, "value") {
						vCfg.Value = attr.Value
					}
				}
				if vCfg.Name != "" {
					vars = append(vars, vCfg)
				}
			} else if elemName == "database" {
				var dbCfg DatabaseConfig
				for _, attr := range se.Attr {
					if strings.EqualFold(attr.Name.Local, "name") {
						dbCfg.Name = attr.Value
					} else if strings.EqualFold(attr.Name.Local, "driver") || strings.EqualFold(attr.Name.Local, "type") {
						dbCfg.Driver = strings.ToLower(attr.Value)
					} else if strings.EqualFold(attr.Name.Local, "connection_string") {
						dbCfg.ConnectionString = attr.Value
					}
				}
				if dbCfg.Driver == "" {
					dbCfg.Driver = "sqlserver"
				}
				if dbCfg.Name != "" && dbCfg.ConnectionString != "" {
					dbs = append(dbs, dbCfg)
				}
			} else if elemName == "scripts" || elemName == "config" || elemName == "variables" || elemName == "databases" {
				continue
			} else {
				node, err := parseNodeElement(decoder, se, &scriptIndex)
				if err != nil {
					return nil, nil, nil, err
				}
				if node != nil {
					nodes = append(nodes, *node)
				}
			}
		}
	}

	return vars, dbs, nodes, nil
}

func parseNodeElement(decoder *xml.Decoder, se xml.StartElement, scriptIndex *int) (*PipelineNode, error) {
	elemName := strings.ToLower(se.Name.Local)

	switch elemName {
	case "script":
		lang, scriptID, dbName, targetDB, targetTable, varName, outputVar := "", "", "", "", "", "", ""
		batchSize := 0

		for _, attr := range se.Attr {
			attrName := strings.ToLower(attr.Name.Local)
			switch attrName {
			case "language", "lang":
				lang = strings.ToLower(attr.Value)
			case "id":
				scriptID = attr.Value
			case "db", "database":
				dbName = attr.Value
			case "target_db", "target_database":
				targetDB = attr.Value
			case "target_table":
				targetTable = attr.Value
			case "batch_size":
				if b, err := strconv.Atoi(attr.Value); err == nil {
					batchSize = b
				}
			case "variable", "var":
				varName = attr.Value
			case "output_var", "output_variable", "out_var":
				outputVar = attr.Value
			}
		}

		if lang == "go" || lang == "sql" {
			if scriptID == "" {
				scriptID = fmt.Sprintf("script_%d", *scriptIndex)
				(*scriptIndex)++
			}
			var content string
			if err := decoder.DecodeElement(&content, &se); err != nil {
				return nil, err
			}
			return &PipelineNode{
				Kind: NodeScript,
				Script: &ScriptItem{
					ID:          scriptID,
					Language:    lang,
					DBName:      dbName,
					TargetDB:    targetDB,
					TargetTable: targetTable,
					BatchSize:   batchSize,
					VarName:     varName,
					OutputVar:   outputVar,
					Code:        strings.TrimSpace(content),
				},
			}, nil
		}

	case "group":
		var groupID, ifVar, ifEquals, condition string
		for _, attr := range se.Attr {
			attrName := strings.ToLower(attr.Name.Local)
			switch attrName {
			case "id":
				groupID = attr.Value
			case "if_var", "var":
				ifVar = attr.Value
			case "if_val", "if_equals", "equals", "value":
				ifEquals = attr.Value
			case "condition", "cond":
				condition = attr.Value
			}
		}
		if condition != "" && ifVar == "" {
			ifVar = condition
		}

		children, err := parseChildrenUntil(decoder, "group", scriptIndex)
		if err != nil {
			return nil, err
		}

		return &PipelineNode{
			Kind:     NodeGroup,
			GroupID:  groupID,
			IfVar:    ifVar,
			IfEquals: ifEquals,
			Children: children,
		}, nil

	case "parallel":
		maxThreads := 0
		for _, attr := range se.Attr {
			attrName := strings.ToLower(attr.Name.Local)
			if attrName == "max_threads" || attrName == "threads" || attrName == "concurrency" {
				if t, err := strconv.Atoi(attr.Value); err == nil {
					maxThreads = t
				}
			}
		}

		children, err := parseChildrenUntil(decoder, "parallel", scriptIndex)
		if err != nil {
			return nil, err
		}

		return &PipelineNode{
			Kind:       NodeParallel,
			MaxThreads: maxThreads,
			Children:   children,
		}, nil

	case "if":
		var ifVar, ifEquals, condition string
		for _, attr := range se.Attr {
			attrName := strings.ToLower(attr.Name.Local)
			switch attrName {
			case "var", "if_var":
				ifVar = attr.Value
			case "equals", "val", "value", "if_val", "if_equals":
				ifEquals = attr.Value
			case "condition", "cond":
				condition = attr.Value
			}
		}
		if condition != "" && ifVar == "" {
			ifVar = condition
		}

		thenNodes, elseNodes, err := parseIfChildren(decoder, scriptIndex)
		if err != nil {
			return nil, err
		}

		return &PipelineNode{
			Kind:      NodeIf,
			IfVar:     ifVar,
			IfEquals:  ifEquals,
			Children:  thenNodes,
			ElseNodes: elseNodes,
		}, nil

	case "foreach", "loop":
		var foreachID, lang, dbName, varName string
		for _, attr := range se.Attr {
			attrName := strings.ToLower(attr.Name.Local)
			switch attrName {
			case "id":
				foreachID = attr.Value
			case "language", "lang":
				lang = strings.ToLower(attr.Value)
			case "db", "database":
				dbName = attr.Value
			case "variable", "var":
				varName = attr.Value
			}
		}

		if lang == "" {
			lang = "sql"
		}
		if foreachID == "" {
			foreachID = fmt.Sprintf("foreach_%d", *scriptIndex)
			(*scriptIndex)++
		}

		children, err := parseChildrenUntil(decoder, elemName, scriptIndex)
		if err != nil {
			return nil, err
		}

		driverScript := &ScriptItem{
			ID:       fmt.Sprintf("%s_driver", foreachID),
			Language: lang,
			DBName:   dbName,
			VarName:  varName,
		}

		return &PipelineNode{
			Kind:          NodeForEach,
			GroupID:       foreachID,
			ForEachScript: driverScript,
			Children:      children,
		}, nil
	}

	return nil, nil
}

func parseChildrenUntil(decoder *xml.Decoder, closingTag string, scriptIndex *int) ([]PipelineNode, error) {
	var nodes []PipelineNode
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("unexpected EOF waiting for </%s>", closingTag)
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, closingTag) {
				return nodes, nil
			}
		case xml.StartElement:
			node, err := parseNodeElement(decoder, t, scriptIndex)
			if err != nil {
				return nil, err
			}
			if node != nil {
				nodes = append(nodes, *node)
			}
		}
	}
}

func parseIfChildren(decoder *xml.Decoder, scriptIndex *int) ([]PipelineNode, []PipelineNode, error) {
	var thenNodes []PipelineNode
	var elseNodes []PipelineNode

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil, nil, fmt.Errorf("unexpected EOF inside <if>")
		}
		if err != nil {
			return nil, nil, err
		}

		switch t := tok.(type) {
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "if") {
				return thenNodes, elseNodes, nil
			}
		case xml.StartElement:
			elemName := strings.ToLower(t.Name.Local)
			if elemName == "then" {
				nodes, err := parseChildrenUntil(decoder, "then", scriptIndex)
				if err != nil {
					return nil, nil, err
				}
				thenNodes = append(thenNodes, nodes...)
			} else if elemName == "else" {
				nodes, err := parseChildrenUntil(decoder, "else", scriptIndex)
				if err != nil {
					return nil, nil, err
				}
				elseNodes = append(elseNodes, nodes...)
			} else {
				node, err := parseNodeElement(decoder, t, scriptIndex)
				if err != nil {
					return nil, nil, err
				}
				if node != nil {
					thenNodes = append(thenNodes, *node)
				}
			}
		}
	}
}

// -----------------------------------------------------------------------------
// SEMANTIC AST VALIDATION PASS
// -----------------------------------------------------------------------------

func validateAST(nodes []PipelineNode, registeredDBs []DatabaseConfig) error {
	var errs []string
	knownIDs := make(map[string]bool)

	definedDBs := make(map[string]bool)
	for _, db := range registeredDBs {
		definedDBs[db.Name] = true
	}

	var inspect func(nodes []PipelineNode)
	inspect = func(nodes []PipelineNode) {
		for _, node := range nodes {
			switch node.Kind {
			case NodeScript:
				s := node.Script
				if s.ID != "" {
					if knownIDs[s.ID] {
						errs = append(errs, fmt.Sprintf("duplicate script ID found: '%s'", s.ID))
					}
					knownIDs[s.ID] = true
				}

				if s.Language == "sql" && s.DBName != "" {
					if !definedDBs[s.DBName] {
						errs = append(errs, fmt.Sprintf("script '%s' references unregistered database '%s'", s.ID, s.DBName))
					}
				}
				if s.TargetDB != "" && !definedDBs[s.TargetDB] {
					errs = append(errs, fmt.Sprintf("script '%s' target_db references unregistered database '%s'", s.ID, s.TargetDB))
				}

				if strings.TrimSpace(s.Code) == "" && s.VarName == "" {
					errs = append(errs, fmt.Sprintf("script '%s' has an empty body and no driver variable", s.ID))
				}

			case NodeParallel:
				if node.MaxThreads < 0 {
					errs = append(errs, fmt.Sprintf("parallel block has invalid max_threads: %d", node.MaxThreads))
				}
				if len(node.Children) == 0 {
					errs = append(errs, "parallel block contains no child scripts")
				}
				inspect(node.Children)

			case NodeIf:
				if node.IfVar == "" {
					errs = append(errs, "<if> condition tag is missing a target variable name")
				}
				if len(node.Children) == 0 && len(node.ElseNodes) == 0 {
					errs = append(errs, "<if> block contains neither <then> nor <else> child nodes")
				}
				inspect(node.Children)
				inspect(node.ElseNodes)

			case NodeForEach:
				if node.ForEachScript != nil && node.ForEachScript.DBName != "" {
					if !definedDBs[node.ForEachScript.DBName] {
						errs = append(errs, fmt.Sprintf("foreach '%s' driver query references unregistered database '%s'", node.GroupID, node.ForEachScript.DBName))
					}
				}
				inspect(node.Children)

			case NodeGroup:
				inspect(node.Children)
			}
		}
	}

	inspect(nodes)

	if len(errs) > 0 {
		return fmt.Errorf("XML semantic validation failed:\n - %s", strings.Join(errs, "\n - "))
	}
	return nil
}

// -----------------------------------------------------------------------------
// ENGINE INITIALIZATION & EXECUTION
// -----------------------------------------------------------------------------

func initVariables(configs []VariableConfig) error {
	varMu.Lock()
	defer varMu.Unlock()

	for _, cfg := range configs {
		switch cfg.Type {
		case "int", "integer":
			val, err := strconv.Atoi(cfg.Value)
			if err != nil {
				return fmt.Errorf("invalid int value '%s' for variable '%s'", cfg.Value, cfg.Name)
			}
			varRegistry[cfg.Name] = val
		case "bool", "boolean":
			val, err := strconv.ParseBool(cfg.Value)
			if err != nil {
				return fmt.Errorf("invalid bool value '%s' for variable '%s'", cfg.Value, cfg.Name)
			}
			varRegistry[cfg.Name] = val
		case "float", "double", "float64":
			val, err := strconv.ParseFloat(cfg.Value, 64)
			if err != nil {
				return fmt.Errorf("invalid float value '%s' for variable '%s'", cfg.Value, cfg.Name)
			}
			varRegistry[cfg.Name] = val
		default:
			varRegistry[cfg.Name] = cfg.Value
		}
	}
	return nil
}

func initDatabases(configs []DatabaseConfig) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	varMu.RLock()
	defer varMu.RUnlock()

	for _, cfg := range configs {
		connStr := cfg.ConnectionString

		for name, val := range varRegistry {
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

		dbRegistry[cfg.Name] = DBHandle{
			Conn:   dbConn,
			Driver: driverName,
		}
	}
	return nil
}

func closeDatabases() {
	dbMu.Lock()
	defer dbMu.Unlock()

	for name, handle := range dbRegistry {
		handle.Conn.Close()
		delete(dbRegistry, name)
	}
}

func evalCondition(varName string, expectedVal string) bool {
	varName = strings.TrimSpace(varName)
	expectedVal = strings.TrimSpace(expectedVal)

	if expectedVal == "" && (strings.Contains(varName, "==") || strings.Contains(varName, "!=")) {
		if strings.Contains(varName, "==") {
			parts := strings.SplitN(varName, "==", 2)
			varName = strings.TrimSpace(parts[0])
			expectedVal = strings.TrimSpace(parts[1])
		} else if strings.Contains(varName, "!=") {
			parts := strings.SplitN(varName, "!=", 2)
			vName := strings.TrimSpace(parts[0])
			eVal := strings.TrimSpace(parts[1])
			return !evalCondition(vName, eVal)
		}
	}

	val := GetVar(varName)
	if val == nil {
		return false
	}

	if expectedVal != "" {
		expectedVal = strings.Trim(expectedVal, "'\"")
		actualStr := strings.TrimSpace(fmt.Sprintf("%v", val))
		return strings.EqualFold(actualStr, expectedVal)
	}

	switch v := val.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case float64:
		return v != 0.0
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes" || s == "y"
	default:
		return false
	}
}

func executeSQLScript(dbName string, queryStr string) (resultsString string, rawOutput string, err error) {
	if dbName == "" {
		return "", "", fmt.Errorf("missing 'db' attribute on <script language=\"sql\"> tag")
	}

	varMu.RLock()
	for name, val := range varRegistry {
		placeholder := fmt.Sprintf("{{%s}}", name)
		queryStr = strings.ReplaceAll(queryStr, placeholder, fmt.Sprintf("%v", val))
	}
	varMu.RUnlock()

	dbConn, err := GetDB(dbName)
	if err != nil {
		return "", "", err
	}

	rows, err := dbConn.Query(queryStr)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", "", err
	}

	var dataBuf bytes.Buffer
	dataBuf.WriteString(strings.Join(cols, "\t") + "\n")

	rowCount := 0
	var lastRowStrs []string

	for rows.Next() {
		rowCount++
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return dataBuf.String(), "", fmt.Errorf("row scan error: %w", err)
		}

		rowStrs := make([]string, len(cols))
		for i, v := range vals {
			if v == nil {
				rowStrs[i] = "NULL"
			} else if b, ok := v.([]byte); ok {
				rowStrs[i] = string(b)
			} else {
				rowStrs[i] = fmt.Sprintf("%v", v)
			}
		}
		lastRowStrs = rowStrs
		dataBuf.WriteString(strings.Join(rowStrs, "\t") + "\n")
	}

	if err := rows.Err(); err != nil {
		return dataBuf.String(), "", err
	}

	var logBuf bytes.Buffer
	logBuf.WriteString(dataBuf.String())
	logBuf.WriteString(fmt.Sprintf("\n(%d row(s) returned)\n", rowCount))

	if rowCount == 1 && len(cols) == 1 {
		rawOutput = strings.TrimSpace(lastRowStrs[0])
	} else {
		rawOutput = strings.TrimSpace(dataBuf.String())
	}

	return logBuf.String(), rawOutput, nil
}

func storeScriptOutput(outputVar string, output string) {
	varMu.Lock()
	defer varMu.Unlock()

	varRegistry["LAST_OUTPUT"] = output
	if outputVar != "" {
		varRegistry[outputVar] = output
	}
}

func appendResult(results *[]ScriptResult, res ScriptResult) {
	resultsMu.Lock()
	defer resultsMu.Unlock()
	*results = append(*results, res)
}

func executeScriptNode(script ScriptItem, results *[]ScriptResult) bool {
	codeToEval := script.Code

	if script.VarName != "" {
		varMu.RLock()
		val, ok := varRegistry[script.VarName]
		varMu.RUnlock()

		if ok && val != nil {
			strVal := strings.TrimSpace(fmt.Sprintf("%v", val))
			if strVal != "" && strVal != "<nil>" {
				codeToEval = strVal
			}
		}
	}

	res := ScriptResult{ScriptID: script.ID}

	if script.Language == "sql" {
		if script.TargetTable != "" {
			targetDB := script.TargetDB
			if targetDB == "" {
				targetDB = script.DBName
			}

			copied, err := StreamETL(script.DBName, codeToEval, targetDB, script.TargetTable, script.BatchSize)
			if err != nil {
				res.ReturnCode = err.Error()
				appendResult(results, res)
				return true
			}
			res.ReturnCode = 0
			res.ResultsString = fmt.Sprintf("Streamed %d row(s) directly to %s.%s\n", copied, targetDB, script.TargetTable)
			storeScriptOutput(script.OutputVar, fmt.Sprintf("%d", copied))
			appendResult(results, res)
		} else {
			logOutput, rawOutput, err := executeSQLScript(script.DBName, codeToEval)
			res.ResultsString = logOutput
			if err != nil {
				res.ReturnCode = err.Error()
				appendResult(results, res)
				return true
			}
			res.ReturnCode = 0
			storeScriptOutput(script.OutputVar, rawOutput)
			appendResult(results, res)
		}

	} else if script.Language == "go" {
		var outBuf bytes.Buffer
		i := interp.New(interp.Options{
			Stdout: &outBuf,
			Stderr: &outBuf,
		})

		if err := i.Use(stdlib.Symbols); err != nil {
			res.ReturnCode = 1
			res.ResultsString = fmt.Sprintf("Failed to load stdlib symbols: %v", err)
			appendResult(results, res)
			return true
		}

		dbExports := map[string]reflect.Value{
			"Get":       reflect.ValueOf(GetDB),
			"StreamETL": reflect.ValueOf(StreamETL),
		}

		varsExports := map[string]reflect.Value{
			"Get":       reflect.ValueOf(GetVar),
			"GetString": reflect.ValueOf(GetVarString),
			"GetInt":    reflect.ValueOf(GetVarInt),
			"GetBool":   reflect.ValueOf(GetVarBool),
			"GetFloat":  reflect.ValueOf(GetVarFloat),
		}

		if err := i.Use(interp.Exports{
			"host/db/db":          dbExports,
			"host/db/db/db":       dbExports,
			"host/vars/vars":      varsExports,
			"host/vars/vars/vars": varsExports,
		}); err != nil {
			res.ReturnCode = 1
			res.ResultsString = fmt.Sprintf("Failed to export packages: %v", err)
			appendResult(results, res)
			return true
		}

		_, err := i.Eval(codeToEval)
		if err != nil {
			res.ReturnCode = err.Error()
			res.ResultsString = outBuf.String()
			appendResult(results, res)
			return true
		}

		res.ReturnCode = 0
		res.ResultsString = outBuf.String()
		storeScriptOutput(script.OutputVar, strings.TrimSpace(outBuf.String()))
		appendResult(results, res)
	}
	return false
}

func executeForEachNode(node PipelineNode, results *[]ScriptResult) bool {
	script := node.ForEachScript
	if script == nil {
		return false
	}

	codeToEval := script.Code
	if script.VarName != "" {
		varMu.RLock()
		val, ok := varRegistry[script.VarName]
		varMu.RUnlock()

		if ok && val != nil {
			strVal := strings.TrimSpace(fmt.Sprintf("%v", val))
			if strVal != "" && strVal != "<nil>" {
				codeToEval = strVal
			}
		}
	}

	if script.Language == "sql" {
		varMu.RLock()
		for name, val := range varRegistry {
			placeholder := fmt.Sprintf("{{%s}}", name)
			codeToEval = strings.ReplaceAll(codeToEval, placeholder, fmt.Sprintf("%v", val))
		}
		varMu.RUnlock()

		dbConn, err := GetDB(script.DBName)
		if err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			appendResult(results, res)
			return true
		}

		rows, err := dbConn.Query(codeToEval)
		if err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			appendResult(results, res)
			return true
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			appendResult(results, res)
			return true
		}

		loopIdx := 0
		for rows.Next() {
			vals := make([]interface{}, len(cols))
			valPtrs := make([]interface{}, len(cols))
			for i := range vals {
				valPtrs[i] = &vals[i]
			}

			if err := rows.Scan(valPtrs...); err != nil {
				res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
				appendResult(results, res)
				return true
			}

			varMu.Lock()
			varRegistry["LOOP_INDEX"] = loopIdx
			for i, col := range cols {
				var strVal string
				if vals[i] == nil {
					strVal = "NULL"
				} else if b, ok := vals[i].([]byte); ok {
					strVal = string(b)
				} else {
					strVal = fmt.Sprintf("%v", vals[i])
				}
				varRegistry[col] = strVal
				varRegistry[strings.ToLower(col)] = strVal
				varRegistry[strings.ToUpper(col)] = strVal
			}
			varMu.Unlock()

			if hasErr := executeNodes(node.Children, results); hasErr {
				return true
			}

			loopIdx++
		}

		if err := rows.Err(); err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			appendResult(results, res)
			return true
		}
	}
	return false
}

func executeParallelNode(node PipelineNode, results *[]ScriptResult) bool {
	maxThreads := node.MaxThreads
	if maxThreads <= 0 {
		maxThreads = 4
	}

	sem := make(chan struct{}, maxThreads)
	var wg sync.WaitGroup
	var hasErr atomic.Bool

	for _, child := range node.Children {
		wg.Add(1)
		sem <- struct{}{}

		go func(childNode PipelineNode) {
			defer wg.Done()
			defer func() { <-sem }()

			if hasErr.Load() {
				return
			}

			var localResults []ScriptResult
			childErr := executeNodes([]PipelineNode{childNode}, &localResults)

			resultsMu.Lock()
			*results = append(*results, localResults...)
			resultsMu.Unlock()

			if childErr {
				hasErr.Store(true)
			}
		}(child)
	}

	wg.Wait()
	return hasErr.Load()
}

func executeNodes(nodes []PipelineNode, results *[]ScriptResult) bool {
	for _, node := range nodes {
		switch node.Kind {
		case NodeScript:
			if hasErr := executeScriptNode(*node.Script, results); hasErr {
				return true
			}

		case NodeGroup:
			if node.IfVar != "" {
				if !evalCondition(node.IfVar, node.IfEquals) {
					continue
				}
			}
			if hasErr := executeNodes(node.Children, results); hasErr {
				return true
			}

		case NodeParallel:
			if hasErr := executeParallelNode(node, results); hasErr {
				return true
			}

		case NodeIf:
			condPassed := evalCondition(node.IfVar, node.IfEquals)
			var target []PipelineNode
			if condPassed {
				target = node.Children
			} else {
				target = node.ElseNodes
			}
			if hasErr := executeNodes(target, results); hasErr {
				return true
			}

		case NodeForEach:
			if hasErr := executeForEachNode(node, results); hasErr {
				return true
			}
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// MAIN ENTRY POINT
// -----------------------------------------------------------------------------

func main() {
	filePath := flag.String("file", "scripts.xml", "Path to XML file containing scripts and databases")
	xsdPath := flag.String("xsd", "", "Path to XSD file for schema validation (optional)")
	configPath := flag.String("config", "", "Optional path to CONFIG.xml file containing variable overrides")
	validateOnly := flag.Bool("validate", false, "Validate XML schema and structure without executing pipeline")
	flag.Parse()

	// 1. Optional XSD Validation Pass (runs xmllint if -xsd flag is provided)
	if *xsdPath != "" {
		if err := validateXSD(*filePath, *xsdPath); err != nil {
			outputJSON([]ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: err.Error(),
			}})
			os.Exit(1)
		}
	}

	// 2. Load and Parse XML File
	fileBytes, err := os.ReadFile(*filePath)
	if err != nil {
		outputJSON([]ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("Error reading script file: %v", err),
		}})
		os.Exit(1)
	}

	varConfigs, dbConfigs, nodes, err := parseXMLConfig(fileBytes)
	if err != nil {
		outputJSON([]ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("XML parsing error in script file: %v", err),
		}})
		os.Exit(1)
	}

	if *configPath != "" {
		configBytes, err := os.ReadFile(*configPath)
		if err != nil {
			outputJSON([]ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error reading config override file: %v", err),
			}})
			os.Exit(1)
		}

		overrideVars, overrideDBs, _, err := parseXMLConfig(configBytes)
		if err != nil {
			outputJSON([]ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("XML parsing error in config file: %v", err),
			}})
			os.Exit(1)
		}

		varConfigs = append(varConfigs, overrideVars...)
		dbConfigs = append(dbConfigs, overrideDBs...)
	}

	// 3. Semantic AST Validation Pass
	if err := validateAST(nodes, dbConfigs); err != nil {
		outputJSON([]ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}

	if *validateOnly {
		outputJSON([]ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    0,
			ResultsString: "XML pipeline schema (XSD) and AST structure are valid.",
		}})
		os.Exit(0)
	}

	// 4. Initialize State and Execute Pipeline
	if err := initVariables(varConfigs); err != nil {
		outputJSON([]ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}

	if err := initDatabases(dbConfigs); err != nil {
		outputJSON([]ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}
	defer closeDatabases()

	var results []ScriptResult
	hasError := executeNodes(nodes, &results)

	outputJSON(results)

	if hasError {
		os.Exit(1)
	}
}

func outputJSON(res any) {
	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}