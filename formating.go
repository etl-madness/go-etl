package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/etl-madness/flow"
)

type PrintableResult struct {
	ScriptID      string      `json:"script_id"`
	ReturnCode    any         `json:"return_code"`
	ResultsString interface{} `json:"results_string"`
}

func formatLineReturns(s string) string {
	cleaned := strings.ReplaceAll(s, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, `\r\n`, "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	return cleaned
}

func outputRawJSON(res any) {
	jsonBytes, err := json.Marshal(res)
	if err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}
	// Pass JSON output through line return cleanup before printing to stdout
	fmt.Println(formatLineReturns(string(jsonBytes)))
}
func outputJSON(res []flow.ScriptResult) {
	var printable []PrintableResult

	for _, r := range res {
		cleanStr := formatLineReturns(r.ResultsString)
		var val interface{} = cleanStr

		// Embed as raw JSON object if ResultsString contains valid JSON
		if json.Valid([]byte(cleanStr)) {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(cleanStr), &raw); err == nil {
				val = raw
			}
		}

		printable = append(printable, PrintableResult{
			ScriptID:      r.ScriptID,
			ReturnCode:    r.ReturnCode,
			ResultsString: val,
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(printable); err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}

	// Unescape JSON-encoded literal newline characters (\n and \r\n) inside the marshaled buffer
	// using a temporary placeholder to preserve double-escaped path characters (e.g., C:\\folder)
	outputStr := buf.String()
	const backslashPlaceholder = "___DOUBLE_BACKSLASH___"
	outputStr = strings.ReplaceAll(outputStr, `\\`, backslashPlaceholder)
	outputStr = strings.ReplaceAll(outputStr, `\r\n`, "\n")
	outputStr = strings.ReplaceAll(outputStr, `\n`, "\n")
	outputStr = strings.ReplaceAll(outputStr, `\r`, "")
	outputStr = strings.ReplaceAll(outputStr, backslashPlaceholder, `\\`)

	// Loop through each line character by line break and print individually
	lines := strings.Split(outputStr, "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue // Skip trailing empty line from encoder buffer
		}
		fmt.Println(line)
	}
}

func outputText(res []flow.ScriptResult) {
	for _, r := range res {
		fmt.Printf("ScriptID: %s\nReturnCode: %d\nResultsString: %s\n\n", r.ScriptID, r.ReturnCode, r.ResultsString)
	}
}
func outputMarkdownTable(res []flow.ScriptResult) {
	fmt.Println("| Script ID | Return Code | Results |")
	fmt.Println("| :--- | :--- | :--- |")
	for _, r := range res {
		// Escape newlines and pipe symbols to prevent table formatting breaks
		cleanResults := strings.ReplaceAll(r.ResultsString, "\n", "<br>")
		cleanResults = strings.ReplaceAll(cleanResults, "|", "\\|")
		fmt.Printf("| %s | %d | %s |\n", r.ScriptID, r.ReturnCode, cleanResults)
	}
}
