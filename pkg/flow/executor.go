package flow

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type ScriptResult struct {
	ScriptID      string `json:"script_id"`
	ReturnCode    any    `json:"return_code"`
	ResultsString string `json:"results_string"`
}

type Executor struct {
	registry  *Registry
	resultsMu sync.Mutex
}

func NewExecutor(r *Registry) *Executor {
	return &Executor{
		registry: r,
	}
}

func (e *Executor) Execute(nodes []PipelineNode) ([]ScriptResult, error) {
	var results []ScriptResult
	hasErr := e.executeNodes(nodes, &results)
	if hasErr {
		return results, fmt.Errorf("pipeline execution encountered errors")
	}
	return results, nil
}

func (e *Executor) appendResult(results *[]ScriptResult, res ScriptResult) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()
	*results = append(*results, res)
}

func (e *Executor) evalCondition(varName string, expectedVal string) bool {
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
			return !e.evalCondition(vName, eVal)
		}
	}

	val := e.registry.GetVar(varName)
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

func (e *Executor) executeSQLScript(dbName string, queryStr string) (resultsString string, rawOutput string, err error) {
	if dbName == "" {
		return "", "", fmt.Errorf("missing 'db' attribute on <script language=\"sql\"> tag")
	}

	variables := e.registry.CopyVariables()
	for name, val := range variables {
		placeholder := fmt.Sprintf("{{%s}}", name)
		queryStr = strings.ReplaceAll(queryStr, placeholder, fmt.Sprintf("%v", val))
	}

	dbConn, err := e.registry.GetDB(dbName)
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

func (e *Executor) storeScriptOutput(outputVar string, output string) {
	e.registry.SetVar("LAST_OUTPUT", output)
	if outputVar != "" {
		e.registry.SetVar(outputVar, output)
	}
}

func (e *Executor) executeScriptNode(script ScriptItem, results *[]ScriptResult) bool {
	codeToEval := script.Code

	if script.VarName != "" {
		val := e.registry.GetVar(script.VarName)
		if val != nil {
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

			copied, err := StreamETL(e.registry, script.DBName, codeToEval, targetDB, script.TargetTable, script.BatchSize)
			if err != nil {
				res.ReturnCode = err.Error()
				e.appendResult(results, res)
				return true
			}
			res.ReturnCode = 0
			res.ResultsString = fmt.Sprintf("Streamed %d row(s) directly to %s.%s\n", copied, targetDB, script.TargetTable)
			e.storeScriptOutput(script.OutputVar, fmt.Sprintf("%d", copied))
			e.appendResult(results, res)
		} else {
			logOutput, rawOutput, err := e.executeSQLScript(script.DBName, codeToEval)
			res.ResultsString = logOutput
			if err != nil {
				res.ReturnCode = err.Error()
				e.appendResult(results, res)
				return true
			}
			res.ReturnCode = 0
			e.storeScriptOutput(script.OutputVar, rawOutput)
			e.appendResult(results, res)
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
			e.appendResult(results, res)
			return true
		}

		dbExports := map[string]reflect.Value{
			"Get": reflect.ValueOf(e.registry.GetDB),
			"StreamETL": reflect.ValueOf(func(srcDB, query, dstDB, targetTable string, batchSize int) (int64, error) {
				return StreamETL(e.registry, srcDB, query, dstDB, targetTable, batchSize)
			}),
		}

		varsExports := map[string]reflect.Value{
			"Get":       reflect.ValueOf(e.registry.GetVar),
			"GetString": reflect.ValueOf(e.registry.GetVarString),
			"GetInt":    reflect.ValueOf(e.registry.GetVarInt),
			"GetBool":   reflect.ValueOf(e.registry.GetVarBool),
			"GetFloat":  reflect.ValueOf(e.registry.GetVarFloat),
		}

		if err := i.Use(interp.Exports{
			"host/db/db":          dbExports,
			"host/db/db/db":       dbExports,
			"host/vars/vars":      varsExports,
			"host/vars/vars/vars": varsExports,
		}); err != nil {
			res.ReturnCode = 1
			res.ResultsString = fmt.Sprintf("Failed to export packages: %v", err)
			e.appendResult(results, res)
			return true
		}

		_, err := i.Eval(codeToEval)
		if err != nil {
			res.ReturnCode = err.Error()
			res.ResultsString = outBuf.String()
			e.appendResult(results, res)
			return true
		}

		res.ReturnCode = 0
		res.ResultsString = outBuf.String()
		e.storeScriptOutput(script.OutputVar, strings.TrimSpace(outBuf.String()))
		e.appendResult(results, res)
	}
	return false
}

func (e *Executor) executeForEachNode(node PipelineNode, results *[]ScriptResult) bool {
	script := node.ForEachScript
	if script == nil {
		return false
	}

	codeToEval := script.Code
	if script.VarName != "" {
		val := e.registry.GetVar(script.VarName)
		if val != nil {
			strVal := strings.TrimSpace(fmt.Sprintf("%v", val))
			if strVal != "" && strVal != "<nil>" {
				codeToEval = strVal
			}
		}
	}

	if script.Language == "sql" {
		variables := e.registry.CopyVariables()
		for name, val := range variables {
			placeholder := fmt.Sprintf("{{%s}}", name)
			codeToEval = strings.ReplaceAll(codeToEval, placeholder, fmt.Sprintf("%v", val))
		}

		dbConn, err := e.registry.GetDB(script.DBName)
		if err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			e.appendResult(results, res)
			return true
		}

		rows, err := dbConn.Query(codeToEval)
		if err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			e.appendResult(results, res)
			return true
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			e.appendResult(results, res)
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
				e.appendResult(results, res)
				return true
			}

			e.registry.SetVar("LOOP_INDEX", loopIdx)
			for i, col := range cols {
				var strVal string
				if vals[i] == nil {
					strVal = "NULL"
				} else if b, ok := vals[i].([]byte); ok {
					strVal = string(b)
				} else {
					strVal = fmt.Sprintf("%v", vals[i])
				}
				e.registry.SetVar(col, strVal)
				e.registry.SetVar(strings.ToLower(col), strVal)
				e.registry.SetVar(strings.ToUpper(col), strVal)
			}

			if hasErr := e.executeNodes(node.Children, results); hasErr {
				return true
			}

			loopIdx++
		}

		if err := rows.Err(); err != nil {
			res := ScriptResult{ScriptID: script.ID, ReturnCode: err.Error()}
			e.appendResult(results, res)
			return true
		}
	}
	return false
}

func (e *Executor) executeWhileNode(node PipelineNode, results *[]ScriptResult) bool {
	iterations := 0
	maxLimit := node.MaxIterations
	if maxLimit <= 0 {
		maxLimit = 1000
	}

	for e.evalCondition(node.IfVar, node.IfEquals) {
		if iterations >= maxLimit {
			res := ScriptResult{
				ScriptID:   node.GroupID,
				ReturnCode: fmt.Sprintf("Exceeded maximum iteration limit (%d)", maxLimit),
			}
			e.appendResult(results, res)
			return true
		}

		e.registry.SetVar("WHILE_INDEX", iterations)

		if hasErr := e.executeNodes(node.Children, results); hasErr {
			return true
		}

		iterations++
	}

	return false
}

func (e *Executor) executeParallelNode(node PipelineNode, results *[]ScriptResult) bool {
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
			childErr := e.executeNodes([]PipelineNode{childNode}, &localResults)

			e.resultsMu.Lock()
			*results = append(*results, localResults...)
			e.resultsMu.Unlock()

			if childErr {
				hasErr.Store(true)
			}
		}(child)
	}

	wg.Wait()
	return hasErr.Load()
}

func (e *Executor) executeNodes(nodes []PipelineNode, results *[]ScriptResult) bool {
	for _, node := range nodes {
		switch node.Kind {
		case NodeScript:
			if hasErr := e.executeScriptNode(*node.Script, results); hasErr {
				return true
			}

		case NodeGroup:
			if node.IfVar != "" {
				if !e.evalCondition(node.IfVar, node.IfEquals) {
					continue
				}
			}
			if hasErr := e.executeNodes(node.Children, results); hasErr {
				return true
			}

		case NodeParallel:
			if hasErr := e.executeParallelNode(node, results); hasErr {
				return true
			}

		case NodeIf:
			condPassed := e.evalCondition(node.IfVar, node.IfEquals)
			var target []PipelineNode
			if condPassed {
				target = node.Children
			} else {
				target = node.ElseNodes
			}
			if hasErr := e.executeNodes(target, results); hasErr {
				return true
			}

		case NodeForEach:
			if hasErr := e.executeForEachNode(node, results); hasErr {
				return true
			}

		case NodeWhile:
			if hasErr := e.executeWhileNode(node, results); hasErr {
				return true
			}
		}
	}
	return false
}
