package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/etl-madness/flow"
)

// Main entry point for the ETL pipeline executor
// Parses command-line flags, validates XML and XSD, initializes the registry, and executes the pipeline.
// Outputs results in JSON format to stdout.
// Command-line example:
// go run main.go -file scripts.xml -xsd schema.xsd -config CONFIG.xml -vars "TargetTable=override_table,Threshold=500" -debug
func main() {
	filePath := flag.String("file", "scripts.xml", "Path to XML file containing scripts and databases")
	xsdPath := flag.String("xsd", "", "Path to XSD file for schema validation (optional)")
	configPath := flag.String("config", "", "Optional path to CONFIG.xml file containing variable overrides")
	validateOnly := flag.Bool("validate", false, "Validate XML schema and structure without executing pipeline")
	varOverrides := flag.String("vars", "", "Comma-separated key=value overrides (e.g. -vars \"TargetTable=override_table,Threshold=500\")")
	debug := flag.Bool("debug", false, "Enable console logging")
	flag.Parse()

	// 1. Optional XSD Validation Pass (runs xmllint if -xsd flag is provided)
	if *xsdPath != "" {
		if err := flow.ValidateXSD(*filePath, *xsdPath); err != nil {
			outputJSON([]flow.ScriptResult{{
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
		outputJSON([]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("Error reading script file: %v", err),
		}})
		os.Exit(1)
	}

	varConfigs, dbConfigs, nodes, err := flow.ParseXMLConfig(fileBytes)
	if err != nil {
		outputJSON([]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("XML parsing error in script file: %v", err),
		}})
		os.Exit(1)
	}

	if *configPath != "" {
		configBytes, err := os.ReadFile(*configPath)
		if err != nil {
			outputJSON([]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error reading config override file: %v", err),
			}})
			os.Exit(1)
		}

		overrideVars, overrideDBs, _, err := flow.ParseXMLConfig(configBytes)
		if err != nil {
			outputJSON([]flow.ScriptResult{{
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
	if err := flow.ValidateAST(nodes, dbConfigs); err != nil {
		outputJSON([]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}

	if *validateOnly {
		outputJSON([]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    0,
			ResultsString: "XML pipeline schema (XSD) and AST structure are valid.",
		}})
		os.Exit(0)
	}

	// 4. Initialize State and Execute Pipeline
	registry := flow.NewRegistry()

	if err := registry.InitVariables(varConfigs); err != nil {
		outputJSON([]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}
	// Example parameter string (could be passed via flag, CLI arg, or env var)

	applyVariableOverrides(registry, *varOverrides)

	if err := registry.InitDatabases(dbConfigs); err != nil {
		outputJSON([]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}
	defer registry.CloseDatabases()

	executor := flow.NewExecutor(registry)
	executor.SetVerbose(*debug)
	results, execErr := executor.Execute(nodes)

	outputJSON(results)

	if execErr != nil {
		os.Exit(1)
	}
}
func applyVariableOverrides(r *flow.Registry, overrideStr string) {
	if strings.TrimSpace(overrideStr) == "" {
		return
	}

	// Split by comma
	pairs := strings.Split(overrideStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split by '=' delimiter (SplitN ensures values containing '=' aren't broken)
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Warning: Skipping malformed override parameter %q (expected format: var=value)\n", pair)
			continue
		}

		key := strings.TrimSpace(parts[0])
		rawVal := strings.TrimSpace(parts[1])

		// Auto-detect types (int, bool, float) or default to string
		var parsedVal interface{} = rawVal
		if i, err := strconv.Atoi(rawVal); err == nil {
			parsedVal = i
		} else if b, err := strconv.ParseBool(rawVal); err == nil {
			parsedVal = b
		} else if f, err := strconv.ParseFloat(rawVal, 64); err == nil {
			parsedVal = f
		}

		// Override the registry value
		r.SetVar(key, parsedVal)
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
