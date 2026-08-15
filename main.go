package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/etl-madness/flow"
)

func main() {
	filePath := flag.String("file", "scripts.xml", "Path to XML file containing scripts and databases")
	xsdPath := flag.String("xsd", "", "Path to XSD file for schema validation (optional)")
	configPath := flag.String("config", "", "Optional path to CONFIG.xml file containing variable overrides")
	validateOnly := flag.Bool("validate", false, "Validate XML schema and structure without executing pipeline")
	varOverrides := flag.String("vars", "", "Comma-separated key=value overrides (e.g. -vars \"TargetTable=override_table,Threshold=500\")")
	debug := flag.Bool("debug", false, "Enable console logging")

	xsltPath := flag.String("xslt", "", "Path to custom XSLT stylesheet (optional)")
	outFile := flag.String("out", "", "Path to output file for transformed XML (optional)")
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
	if *xsltPath != "" {
		xsltBytes, err := os.ReadFile(*xsltPath)
		if err != nil {
			outputJSON([]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error reading XSLT file: %v", err),
			}})
			os.Exit(1)
		}

		// Generate diagram prior to running XSLT transformation
		diagram, err := generateMermaid(fileBytes)
		if err != nil {
			outputJSON([]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Mermaid generation error: %v", err),
			}})
			os.Exit(1)
		}
		
		fileBytes, err = ProcessXSLT(fileBytes, xsltBytes, diagram, filePath)
		if err != nil {
			outputJSON([]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("XSLT processing error: %v", err),
			}})
			os.Exit(1)
		}
		if *outFile != "" {
			os.WriteFile(*outFile, fileBytes, 0644)
		}
		os.Exit(0)
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

func outputJSON(res any) {
	jsonBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}
