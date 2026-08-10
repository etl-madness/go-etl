package flow

import (
	"fmt"
	"strings"
	"sync"
)

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

// StreamETL streams query results line-by-line from a source database into a target database table using parameterized batch inserts.
func StreamETL(r *Registry, srcDBName, queryStr, dstDBName, targetTable string, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 500
	}

	variables := r.CopyVariables()
	for name, val := range variables {
		placeholder := fmt.Sprintf("{{%s}}", name)
		queryStr = strings.ReplaceAll(queryStr, placeholder, fmt.Sprintf("%v", val))
	}

	srcHandle, err := r.GetDBHandle(srcDBName)
	if err != nil {
		return 0, fmt.Errorf("source db error: %w", err)
	}
	srcDB := srcHandle.Conn

	dstHandle, err := r.GetDBHandle(dstDBName)
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

	// MSSQL limit is 2100 parameters.
	// If dstDriver is mssql or sqlserver, we must ensure we don't exceed 2100 parameters.
	if strings.ToLower(dstDriver) == "sqlserver" || strings.ToLower(dstDriver) == "mssql" {
		maxBatchRows := 2100 / len(cols)
		if maxBatchRows <= 0 {
			maxBatchRows = 1
		}
		if batchSize > maxBatchRows {
			batchSize = maxBatchRows
		}
	}
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

	rowChan := make(chan []interface{}, 5000)
	var readErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(rowChan)
		for rows.Next() {
			scans := make([]interface{}, len(cols))
			vals := make([]interface{}, len(cols))
			for i := range scans {
				scans[i] = &vals[i]
			}

			if err := rows.Scan(scans...); err != nil {
				readErr = fmt.Errorf("row scan error: %w", err)
				return
			}

			rowCopy := make([]interface{}, len(cols))
			for i, v := range vals {
				if b, ok := v.([]byte); ok {
					rowCopy[i] = string(b)
				} else {
					rowCopy[i] = v
				}
			}
			rowChan <- rowCopy
		}
		if err := rows.Err(); err != nil {
			readErr = fmt.Errorf("rows iteration error: %w", err)
		}
	}()

	for row := range rowChan {
		batchRows = append(batchRows, row)
		if len(batchRows) >= batchSize {
			if err := flushBatch(); err != nil {
				// Prevent goroutine leak by draining the channel in a separate goroutine
				go func() {
					for range rowChan {
					}
				}()
				wg.Wait()
				return totalInserted, err
			}
		}
	}

	wg.Wait()
	if readErr != nil {
		return totalInserted, readErr
	}

	if err := flushBatch(); err != nil {
		return totalInserted, err
	}

	return totalInserted, nil
}
