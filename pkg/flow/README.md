# Flow Package (`pkg/flow`)

`flow` is a high-performance, modular, and embeddable data pipeline orchestration and stream ETL library for Go. It allows developers to programmatically load, validate, and execute complex pipeline AST nodes (such as loops, parallel batches, and dynamic SQL/Go scripts) from XML configuration files.

---

## Key Features

- **No Global State**: Fully isolated execution contexts (`Registry`) allowing you to run multiple pipelines concurrently in the same process without interference.
- **Dynamic Configuration Decoder**: Built-in support for XML parsers and optional XSD schema schema validations.
- **Direct Streaming ETL**: Copy bulk datasets line-by-line across heterogeneous engines (PostgreSQL, SQLite, MySQL, Oracle, SQL Server) with automatic parameter placeholder syntax correction.
- **Flexible Flow Controls**: Execution structures for Sequential steps, Parallel queues, If/Else branching, ForEach loops, and While loops.
- **Embedded Script Interpreter**: Dynamic runtime Go evaluations via Yaegi with closure-bound environment state injection.

---

## Package API Reference

### 1. Parsing & Validation
```go
// ParseXMLConfig parses a byte stream of XML into structured configuration blocks.
func ParseXMLConfig(xmlData []byte) ([]VariableConfig, []DatabaseConfig, []PipelineNode, error)

// ValidateAST performs semantic structure checks (uniqueness, reference integrity, loop bounds).
func ValidateAST(nodes []PipelineNode, registeredDBs []DatabaseConfig) error

// ValidateXSD invokes 'xmllint' to validate an XML configuration against schema standards.
func ValidateXSD(xmlPath string, xsdPath string) error
```

### 2. State & Context Management
```go
// Registry holds thread-safe variable registries and database connection pools.
type Registry struct { ... }

func NewRegistry() *Registry
func (r *Registry) InitVariables(configs []VariableConfig) error
func (r *Registry) InitDatabases(configs []DatabaseConfig) error
func (r *Registry) CloseDatabases()

// Variables getters & setters
func (r *Registry) SetVar(name string, value interface{})
func (r *Registry) GetVar(name string) interface{}
func (r *Registry) GetVarString(name string) string
func (r *Registry) GetVarInt(name string) int
func (r *Registry) GetVarBool(name string) bool
```

### 3. Pipeline Executor
```go
// Executor orchestrates tree node executions.
type Executor struct { ... }

func NewExecutor(r *Registry) *Executor
func (e *Executor) Execute(nodes []PipelineNode) ([]ScriptResult, error)
```

---

## Quick Start Example

The following example demonstrates how to load, parse, validate, and execute an XML pipeline programmatically from custom Go code.

```go
package main

import (
	"fmt"
	"log"

	"github.com/etl-madness/go-etl/pkg/flow"
)

func main() {
	xmlConfig := []byte(`<?xml version="1.0" encoding="UTF-8"?>
	<pipeline>
		<variables>
			<variable name="TargetTable" value="processed_logs" />
			<variable name="Threshold" type="int" value="100" />
		</variables>
		<databases>
			<database name="sqlite_db" driver="sqlite" connection_string="./mydb.db" />
		</databases>
		<scripts>
			<script id="SetupTable" language="sql" db="sqlite_db">
				CREATE TABLE IF NOT EXISTS processed_logs (id INTEGER PRIMARY KEY, status TEXT);
			</script>
			<script id="VerifyGo" language="go">
				package main
				import (
					"fmt"
					"host/vars/vars"
				)
				func main() {
					tbl := vars.GetString("TargetTable")
					thresh := vars.GetInt("Threshold")
					fmt.Printf("Configured target table: %s with limit: %d\n", tbl, thresh)
				}
			</script>
		</scripts>
	</pipeline>`)

	// 1. Parse XML to Pipeline AST
	varConfigs, dbConfigs, nodes, err := flow.ParseXMLConfig(xmlConfig)
	if err != nil {
		log.Fatalf("Parsing failed: %v", err)
	}

	// 2. Perform semantic checks on the AST
	if err := flow.ValidateAST(nodes, dbConfigs); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	// 3. Instantiate Registry and Register Connection Pools / Variables
	registry := flow.NewRegistry()
	if err := registry.InitVariables(varConfigs); err != nil {
		log.Fatalf("Variables initialization failed: %v", err)
	}
	if err := registry.InitDatabases(dbConfigs); err != nil {
		log.Fatalf("Databases initialization failed: %v", err)
	}
	defer registry.CloseDatabases()

	// 4. Instantiate Executor and run the pipeline
	executor := flow.NewExecutor(registry)
	results, err := executor.Execute(nodes)
	if err != nil {
		log.Fatalf("Execution encountered errors: %v", err)
	}

	// 5. Inspect Results
	fmt.Println("--- Pipeline Execution Results ---")
	for _, res := range results {
		fmt.Printf("Script [%s]: Return Code: %v\n", res.ScriptID, res.ReturnCode)
		if res.ResultsString != "" {
			fmt.Printf("Output:\n%s\n", res.ResultsString)
		}
	}
}
```

---

## Advanced: Shared Context Isolation

Since the state of connections and active variables is entirely held inside the `*flow.Registry` object rather than package globals, you can safely initialize multiple independent registries and run them in concurrent threads or separate executors:

```go
registryA := flow.NewRegistry()
registryB := flow.NewRegistry()

// Run independent pipelines in parallel
go flow.NewExecutor(registryA).Execute(nodesA)
go flow.NewExecutor(registryB).Execute(nodesB)
```

---

## Complete CLI Wrapper Example

For a complete, real-world example of how to orchestrate, validate, and execute pipelines programmatically using this library, check out the root application files:
- [**`main.go`**](../../main.go): The official, production-ready command line driver that utilizes `pkg/flow` to orchestrate multi-engine data pipelines.
- [**Root `README.md`**](../../README.md): The main guide covering command-line options, database configuration files, and comparative architecture layouts.
