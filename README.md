# SSIS-like XML Script Execution Engine (go-flow)

An enterprise-grade, XML-driven multi-database pipeline executor and ETL engine written in Go. This tool enables developers to define complex, high-performance data processing pipelines that seamlessly combine interpreted Go scripts (executed in-memory via Yaegi) and SQL queries across multiple heterogeneous database engines.

With built-in support for environment-specific configuration overrides, strongly-typed variables, parallel execution, iterative looping, logical branching, and streaming cross-database ETL channels, this engine is optimized for robust, fail-fast, and memory-efficient data movements.

---
## Comparison with SQL Server Integration Services (SSIS)

| Feature / Capability | Go XML Pipeline Engine (`go-flow`) | SQL Server Integration Services (SSIS) |
| :--- | :--- | :--- |
| **Architecture & Footprint** | Lightweight, cross-platform compiled Go CLI binary. Zero server installation overhead. | Heavy, server-based ETL runtime (tied to Windows/SQL Server ecosystem and SSIS Catalog). |
| **Interface / Authoring** | Code-first declarative XML configuration (`scripts.xml`) validated against XSD schemas. | Visual GUI drag-and-drop interface via Visual Studio / SSDT. |
| **Cross-Platform Support** | Native Linux, macOS, and Windows support with zero dependencies. | Windows-centric runtime environment (Linux supported via SQL Server on Linux with limitations). |
| **Database Drivers & Cross-DB ETL** | Native pure Go drivers for MSSQL, PostgreSQL, MySQL, SQLite, and Oracle built-in. Direct cross-DB streaming. | Uses OLE DB, ADO.NET, and ODBC driver managers. Cross-DB setup requires explicit connection manager setups. |
| **Control Flow: Looping** | Supported via `<foreach>` (SQL dataset iteration) and `<while>` (condition-based loops with `max_iterations` caps). | Supported via *Foreach Loop Containers* (files, objects) and *For Loop Containers*. |
| **Control Flow: Logic & Branching** | `<if>` / `<then>` / `<else>` blocks and `<group>` blocks with variable condition evaluation. | Precedence Constraints (Success, Failure, Completion) with optional SSIS Expressions. |
| **Control Flow: Parallelism** | `<parallel max_threads="N">` concurrency control with thread-pool workers. | Engine-level parallel execution of disconnected tasks or `EngineThreads` settings in Data Flow. |
| **Custom Code Execution** | Pure Go embedded scripting via **Yaegi** interpreter with full host variable/DB connection exposure. | *Script Task* and *Script Component* using C# or VB.NET. |
| **In-Memory Streaming ETL** | Direct stream ETL (`target_db` / `target_table`) with automatic driver-aware batch parameterization (`@p1`, `$1`, `?`, `:1`). | Pipeline buffer transformation engine (*Data Flow Tasks*) using memory buffers. |
| **In-Memory MSSQL Bulk Copy** |  Direct stream ETL (`target_db` / `target_table`) with customizable options such as batch_size="25000" tablock="true" check_constraints="true" fire_triggers="false" keep_nulls="true". | *Bulk Insert Task* or *Data Flow Task* with `OLE DB Destination` using `Table or View - Fast Load`. |
| **In-Memory Variable Passing** | Dynamic variable passing between scripts via `output_var` or implicit `LAST_OUTPUT`. | SSIS *Variables* with scope and expression evaluation. |
| **Configuration & Overrides** | Standard XML file overrides (`-config`) allowing easy separation of dev/prod settings without env variables. | Project Parameters, Package Parameters, Configuration Files (dtsConfig), and Environment Overrides in SSIS Catalog. |
| **Validation / Quality Gates** | Two-pass gate: Automated **XSD schema validation** (`xmllint`) + **Semantic AST validation** (broken refs, missing DBs) prior to run. | Visual Studio design-time validation and package-level validation phases. |
| **Output & Reporting** | Structured machine-readable **JSON array** outputting return codes, script output strings, and logs to `stdout`. | Logging to SSISDB Catalog tables, Event Viewer, text logs, or SQL Server tables. |
| **CI/CD & Version Control** | Git-friendly text/XML files; runs seamlessly in lightweight Docker containers, GitHub Actions, or Kubernetes jobs. | XML-backed `.dtsx` files (often difficult to diff/merge in Git); requires Visual Studio or ISDeploymentWizard to deploy. |

## Architecture Overview

The engine parses pipeline definitions into an Abstract Syntax Tree (AST), validates the XML schema and logical relationships, and executes the pipeline in strict order or concurrent branches.

```mermaid
graph TD
    XML[scripts.xml] -->|Parse & Merge| Config[CONFIG.xml Overrides]
    Config --> Parse[XML Parser]
    Parse --> XSD{"XSD Validation <br/> via xmllint"}
    XSD -->|Pass| AST{"Semantic AST <br/> Quality Gate"}
    AST -->|Pass| Exec[Pipeline Executor]
    
    subgraph Engines ["Execution Engines"]
        Exec --> SQL[Multi-Engine SQL Query Executor]
        Exec --> Go[Yaegi Go Interpreter]
    end

    SQL -->|Cross-DB Stream| Stream[Streaming ETL Stream]
    Go -->|host/vars & host/db| Context[Shared Pipeline Context]
    
    Exec -->|Fail Fast / Success| Report[Structured JSON Report]
```

---

## Supported Database Engines

The engine natively registers and supports multiple database drivers. You can configure any mixture of the following drivers within the same pipeline:

| Driver Attribute Value | DB Engine | Go Underling Driver | Placeholders |
| :--- | :--- | :--- | :--- |
| `sqlserver`, `mssql` | Microsoft SQL Server | `github.com/microsoft/go-mssqldb` | `@p1, @p2, ...` |
| `postgres`, `postgresql` | PostgreSQL | `github.com/lib/pq` | `$1, $2, ...` |
| `mysql` | MySQL | `github.com/go-sql-driver/mysql` | `?, ?, ...` |
| `sqlite`, `sqlite3` | SQLite | `modernc.org/sqlite` | `?, ?, ...` |
| `oracle` | Oracle Database | `github.com/sijms/go-ora/v2` | `:1, :2, ...` |

> [!NOTE]
> If the `driver` attribute is omitted on a `<database>` definition, it defaults to **`sqlserver`** for backward compatibility.

---

## Supported Script Languages and OS Shell Executions

`flow` supports executing native host shell commands and binaries directly on the operating system without passing through the Go interpreter[cite: 4]. Supported `language` options on `<script>` tags include:


* **`shell`**: Cross-platform default shell (`cmd /C` on Windows, `sh -c` on Linux/macOS)[cite: 4].
* **`cmd`**: Windows Command Prompt (`cmd /C`)[cite: 4].
* **`powershell,pwsh`**: Windows PowerShell (`powershell -NoProfile -NonInteractive -Command`)[cite: 4].
* **`bash,zsh,ksh,csh,tcsh,dash,fish,sh`**: Various Unix shells (`bash -c`, `zsh -c`, etc.)[cite: 4].
* **`dotnet-script`** (or **`csx`**): Executed using C# script files with `dotnet-script` or `dotnet script`. Allows full inline C# execution including external NuGet package references (`#r "nuget: ..."`). This requires the `dotnet-script` tool to be installed on the host machine and the ability to create temporary files.
* ***`go`**: Interpreted Go scripts executed in-memory via the embedded **Yaegi** interpreter. This allows full access to the engine's host APIs (`host/vars`, `host/db`) and dynamic variable passing between scripts.

---

## Feature Highlights

* 🚀 **Heterogeneous Dual-Engine Pipeline**: Run interpreted **Go scripts** dynamically in-memory via Yaegi alongside native **multi-database SQL queries** (Postgres, MySQL, SQLite, Oracle, MSSQL) within a single, unified orchestration.
* 🎛️ **Advanced Control Flow**:
  * **`<if>`/`<then>`/`<else>`**: Robust logical branching based on dynamic variable values.
  * **`<parallel>`**: Execute tasks concurrently with thread-pool size limits (`max_threads`), complete with automatic propagation of errors.
  * **`<foreach>` (or `<loop>`)**: Iterate over query results, automatically setting column values as script variables per iteration.
  * **`<group>`**: Organize related steps with block-level conditional execution.
* 📦 **Environment-Safe Configuration**: Keep core pipelines (`--file`) separate from environment-specific configuration overrides (`--config`) without relying on brittle OS environment variables.
* 🧬 **XSD & AST Quality Gates**:
  * Optional XML Schema (`.xsd`) validation via `xmllint` to ensure syntactic correctness.
  * Comprehensive semantic checks (duplicate script IDs, connection validity, empty code bodies) executed prior to launching any scripts.
* 🌊 **Memory-Safe Streaming ETL**:
  * **Cross-DB Declarative SQL Streaming**: Instantly copy query results between heterogeneous databases (e.g., PostgreSQL query streamed directly to SQLite) without writing Go code by specifying `target_db` and `target_table` on a SQL script block. The engine automatically maps parameters and placeholders based on the destination's driver syntax.
  * **Dynamic Batch Clamping**: Automatically detects destination connection driver types; for SQL Server (`mssql`/`sqlserver`), it automatically clamps the `batch_size` based on column count to strictly respect the 2100 SQL Server parameter limits.
  * **Decoupled Concurrent Stream Buffering**: Built-in concurrent producer-consumer buffering decoupling database read streams from bulk write flushes, resolving stream type corruption and idle connection timeouts.
  * **Programmatic Go Streaming**: Stream millions of records line-by-line using the `db.StreamETL` host API, utilizing parameterized batch inserts to avoid memory bloat and string limits.
* ⏱️ **Execution Time Tracking**: All step results dynamically compute their actual execution durations (returned as a precise formatted `duration` field, e.g. `"245.85ms"`, in the final structured JSON results).
* 🔗 **Dynamic Connection String Templating**: Define variables and automatically inject them into database connection strings using `{{VarName}}` placeholders.
* 🔄 **Cross-Script Data Passing**: Dynamically pass outputs between blocks using `output_var` attributes or the implicit `LAST_OUTPUT` context variable.
* 🛡️ **Fail-Fast sequential execution**: Halts execution immediately if any step encounters a panic, query syntax error, or unhandled Go error, outputting a complete JSON report up to the failure point.

---

## Additional Resources on how go-flow/flow handles transactions and variables.

* [**GO_TO_SQL.md**](./docs/GO_TO_SQL.md): Detailed reference on how to use the `host/db` package for streaming ETL and executing custom SQL queries from Go scripts.
* [**VARIABLES.md**](./docs/VARIABLES.md): Comprehensive guide on variable declaration, type handling, and cross-script variable passing.
* [**TRANSACTIONS.md**](./docs/TRANSACTIONS.md): Comprehensive guide on transaction management in ETL pipelines.
* [**YAEGI_INTERPRETER.md**](./docs/YAEGI_INTERPRETER.md): Detailed reference on how to use the embedded Yaegi Go interpreter for executing interpreted Go scripts and interacting with the engine context.

---

## Installation & Prerequisites

### Prerequisites
* **Go**: Version 1.20 or later.
* **xmllint**: (Optional) Installed and available in your system path to perform XSD schema validation.

### Getting Started

#### Option A: Programmatic Integration (Public Package)
You can import and embed the fully modular `flow` pipeline engine directly inside your custom Go applications:
```bash
go get github.com/etl-madness/flow 
```

See the package-specific [**`github.com/etl-madness/flow`**](https://github.com/etl-madness/flow) for full developer integration guides and APIs!

#### Option B: Standalone CLI Runner

1. Clone or download the repository to your local workspace.
2. Initialize and tidy the Go module dependencies:

```bash
# Verify & clean dependencies
go mod tidy
go build
```

---

## Command Line Interface (CLI)

Run the engine using command line flags to specify your scripts file, schema files, and execution modes:

```bash
# Basic Execution
go run main.go --file pipeline.xml

# Execution with Variable Overrides via Config File
go run main.go --file pipeline.xml --config production_config.xml

# Full Execution with Variable Overrides via Command Line
go run main.go --file pipeline.xml --vars "BulkSize=1000"

# Pipeline Validation Only (Does not execute scripts)
go run main.go --file pipeline.xml --validate

# Console Logging (Additional verbose output to stdout)
go run main.go --file pipeline.xml --debug

# Full Schema Validation and Execution
go run main.go --file pipeline.xml --xsd schema.xsd --config CONFIG.xml

# Generate HTML Documentation of the Pipeline with Flowchart
go run main.go --file pipeline.xml --xslt autodoc.xslt --out pipeline.html

```

### CLI Flag Reference

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--file` | `scripts.xml` | Path to the main XML file containing variables, databases, and scripts. |
| `--config` | `""` | Optional path to an XML file containing environment variable overrides. |
| `--xsd` | `""` | Optional path to an XSD schema file to run an XML validity check via `xmllint`. |
| `--validate`| `false` | When true, validates XML schema and semantic structure, then exits with code 0 without executing. |
| `--vars` | `""` | Comma-separated key=value pairs to override variables (e.g., `-vars "BatchSize=1000,TargetTable=prod_table"`). |
| `--debug` | `false` | Enables verbose console logging for debugging purposes. |
| `--xslt` | `""` | Optional path to an XSLT file to generate HTML documentation of the pipeline. |
| `--out` | `""` | Optional output path for the generated HTML documentation when using `--xslt`. |
| `--gopath` | `os.Getenv("GOPATH")` | GOPATH directory for interpreter package imports. |
---

## Configuration Reference & Cheat Sheet

The configuration is structured into three main blocks inside the root `<pipeline>` element: `<variables>`, `<databases>`, and `<scripts>`.

```mermaid
classDiagram
    class Pipeline {
        Variables variables
        Databases databases
        Scripts scripts
    }
    class Variables {
        Variable[] variable
    }
    class Databases {
        Database[] database
    }
    class Scripts {
        Script[] script
        Group[] group
        Parallel[] parallel
        If[] if
        ForEach[] foreach
    }
    Pipeline --> Variables
    Pipeline --> Databases
    Pipeline --> Scripts
```

### XML Elements & Attributes

#### `<variable>`
Defines typed global variables. Can be placed under `<pipeline>` in both the main file and override configuration.
* `name` (Required): String identifier used to reference the variable.
* `type` (Optional, defaults to `"string"`): Type of variable (`string`, `int`, `integer`, `bool`, `boolean`, `float`, `double`).
* `value` (Required): Default value or override value.

#### `<database>`
Defines a database connection pool.
* `name` (Required): Unique connection alias used in script blocks.
* `driver` (Optional, defaults to `"sqlserver"`): Database driver to use. Supported values are: `sqlserver` (or `mssql`), `postgres` (or `postgresql`), `mysql`, `sqlite` (or `sqlite3`), `oracle`.
* `connection_string` (Required): Database-specific connection string (supports `{{VarName}}` variable expansion; `&` must be escaped as `&amp;`).

#### `<script>`
Executes code using either the SQL or Go engine.
* `id` (Optional, defaults to `script_N`): Step identifier.
* `language` (Required): Language runtime (`sql` or `go`).
* `db` / `database` (Required for SQL scripts): Target database connection alias.
* `target_db` (Optional): Destination database connection alias for declarative SQL streaming.
* `target_table` (Optional): Target table name for declarative SQL-to-SQL streaming.
* `batch_size` (Optional, defaults to `500`): Chunk size for batch inserts during streaming.
* `variable` / `var` (Optional): Variable containing code. If specified, overrides the script CDATA body (CDATA is treated as a fallback).
* `output_var` / `out_var` (Optional): Store query output or script return string in this variable for subsequent pipeline steps.
* `batch_size` (Optional, defaults to `500`): Chunk size for batch inserts during streaming.
* `tablock` (Optional, defaults to `"false"`): Whether to use table-level locking.
* `check_constraints` (Optional, defaults to `"true"`): Whether to enforce check constraints.
* `fire_triggers` (Optional, defaults to `"true"`): Whether to fire triggers.
* `keep_nulls` (Optional, defaults to `"false"`): Whether to keep null values.
  
#### `<group>`
Combines multiple child nodes.
* `id` (Optional): Identifier for the group block.
* `if_var` / `var` (Optional): Variable name to evaluate.
* `if_equals` / `equals` (Optional): Value to check against. Group executes only if they match.

#### `<parallel>`
Spawns child nodes concurrently.
* `max_threads` / `threads` / `concurrency` (Optional, defaults to `4`): Cap on active concurrent worker threads inside this parallel block.

#### `<if>`
Branching block. Must contain `<then>` and/or `<else>` nodes.
* `var` / `if_var` (Required): Variable name to check.
* `equals` / `if_equals` (Optional): Value to check against. If omitted, evaluates the variable as a boolean.
* `condition` / `cond` (Optional): Evaluates conditional operators like `VarName==value` or `VarName!=value`.

#### `<foreach>` (or `<loop>`)
Iterates over rows returned by an engine query.
* `id` (Optional): Foreach identifier.
* `language` (Optional, defaults to `"sql"`): Language of the driving query.
* `db` (Required for SQL driving query): Database connection name.
* `var` (Optional): Variable containing the driving query.

---

## Host Go APIs (Yaegi Context)

Interpreted Go scripts can interact with the engine environment by importing standard host packages.

### Package `host/vars`
Allows scripts to query, parse, and write pipeline variables.
* **`vars.Get(name string) interface{}`**: Retrieves raw variable value.
* **`vars.GetString(name string) string`**: Returns string representation.
* **`vars.GetInt(name string) int`**: Returns integer representation.
* **`vars.GetBool(name string) bool`**: Returns boolean value.
* **`vars.GetFloat(name string) float64`**: Returns floating-point representation.

### Package `host/db`
Allows scripts to fetch active SQL connections and perform bulk streaming operations.
* **`db.Get(name string) (*sql.DB, error)`**: Returns the underlying native SQL Server connection pool (`*sql.DB`) for custom queries.
* **`db.StreamETL(srcDB, query, dstDB, targetTable, batchSize, tablock, checkConstraints, fireTriggers, keepNulls) (int64, error)`**: Efficiently streams rows line-by-line from a source database query, executing chunked batch parameter queries into a target table.
 
---

## Configuration & Pipeline Examples

### 1. Heterogeneous Multi-Database Setup (`multi_db_type_example.xml`)
Demonstrates how to configure and query multiple database servers of different types and perform cross-engine streaming.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<pipeline>
    <variables>
        <variable name="BatchSize" type="int" value="500" />
    </variables>

    <databases>
        <!-- MSSQL (Default driver when omitted) -->
        <database name="mssql_db" driver="sqlserver" connection_string="sqlserver://sa:Password123!@localhost:1433?database=master&amp;trustServerCertificate=true" />

        <!-- PostgreSQL -->
        <database name="postgres_db" driver="postgres" connection_string="postgres://user:password@localhost:5432/mydb?sslmode=disable" />

        <!-- MySQL -->
        <database name="mysql_db" driver="mysql" connection_string="user:password@tcp(127.0.0.1:3306)/mydb" />

        <!-- SQLite3 -->
        <database name="sqlite_db" driver="sqlite3" connection_string="./pipeline_cache.db" />

        <!-- Oracle -->
        <database name="oracle_db" driver="oracle" connection_string="oracle://user:password@localhost:1521/XEPDB1" />
    </databases>

    <scripts>
        <!-- 1. Query PostgreSQL -->
        <script id="query_pg" language="sql" db="postgres_db">
            <![CDATA[
                SELECT id, username FROM users LIMIT 10;
            ]]>
        </script>

        <!-- 2. Direct Cross-Database Stream ETL from PostgreSQL to SQLite -->
        <!-- The engine handles the parameter syntax transformation (PostgreSQL's $N vs SQLite's ?) seamlessly -->
        <script id="stream_pg_to_sqlite" language="sql" db="postgres_db" target_db="sqlite_db" target_table="cached_users" batch_size="100">
            <![CDATA[
                SELECT id, username FROM users;
            ]]>
        </script>

        <!-- 3. Query MySQL -->
        <script id="query_mysql" language="sql" db="mysql_db">
            <![CDATA[
                SELECT id, status FROM orders WHERE status = 'PENDING';
            ]]>
        </script>
    </scripts>
</pipeline>
```

### 2. Variables and Override Configuration (`CONFIG.xml`)
Allows overriding variables based on environment context (e.g., Development vs. Production) without altering the main pipeline XML structure.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<pipeline>
    <!-- Environmental Variable Overrides -->
    <variables>
        <variable name="BatchSize" type="int" value="1000" />
        <variable name="PrimaryDBConnStr" type="string" value="sqlserver://PROD_SERVER:1433?database=master&amp;integrated+security=true&amp;trustServerCertificate=true" />
        <variable name="AnalyticsDBConnStr" type="string" value="sqlserver://PROD_SERVER:1433?database=AnalyticsDB&amp;integrated+security=true&amp;trustServerCertificate=true" />
        
        <!-- Multi-DB Connection String Overrides -->
        <variable name="PostgresDBConnStr" type="string" value="postgres://PROD_USER:PROD_PASS@prod-db:5432/prod_mydb?sslmode=require" />
        <variable name="MySQLDBConnStr" type="string" value="prod_user:prod_pass@tcp(prod-mysql-db:3306)/prod_mydb" />
    </variables>
</pipeline>
```

### 3. Iterative Looping (`foreach_example.xml`)
Queries databases, maps output columns directly into temporary context variables, and executes nested steps once per row.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<pipeline>
    <variables>
        <variable name="PrimaryDBConnStr" type="string" value="sqlserver://PROD_SERVER:1433?database=master&amp;integrated+security=true&amp;trustServerCertificate=true" />
        <variable name="GetActiveRegionsQuery" value="SELECT database_id, name FROM sys.databases" />
    </variables>

    <databases>
        <database name="primary_db" connection_string="{{PrimaryDBConnStr}}" />
    </databases>

    <scripts>
        <foreach id="RegionLoop" language="sql" db="primary_db" var="GetActiveRegionsQuery">
            <group id="ProcessRegionGroup">
                <!-- Access loop columns via curly braces -->
                <script id="LogRegionSQL" language="sql" db="primary_db">
                    <![CDATA[
                    SELECT 'Processing database ID: {{database_id}}, Name: {{name}}' AS current_status;
                    ]]>
                </script>

                <!-- Access loop columns inside Go code via host APIs -->
                <script id="LogRegionGo" language="go">
                    <![CDATA[
                    package main
                    import (
                        "fmt"
                        "host/vars/vars"
                    )
                    func main() {
                        regionID := vars.GetString("database_id")
                        regionName := vars.GetString("name")
                        loopIdx := vars.GetInt("LOOP_INDEX")
                        fmt.Printf("[Iteration %d] Go script processing %s (ID: %s)\n", loopIdx, regionName, regionID)
                    }
                    ]]>
                </script>
            </group>
        </foreach>
    </scripts>
</pipeline>
```

### 4. Concurrency (`parallel_example.xml`)
Implements concurrent task processing with thread-pool size constraints.

```xml
<parallel max_threads="2">
    <!-- Parallel Branch 1 -->
    <script id="Task1_SqlCleanup" language="sql" db="analytics_db">
        <![CDATA[
        SELECT 'Running log cleanup in parallel branch 1...' AS status;
        ]]>
    </script>

    <!-- Parallel Branch 2 -->
    <script id="Task2_GoWorker" language="go">
        <![CDATA[
        package main
        import (
            "fmt"
            "time"
        )
        func main() {
            fmt.Println("Running Go worker thread in parallel branch 2...")
            time.Sleep(100 * time.Millisecond)
        }
        ]]>
    </script>
</parallel>
```

### 5. MSSQL Bulk Copy (`mssql_trunc_copy_table.xml`)

In comparison to an SSIS package executing the same truncate and dataflow, the SSIS package with equivalent functionality processed truncate and bulk copy of 12151 rows in  00:00:04.500.
The example script below run via go-flow executed in 821.5704ms. Your mileage will vary based on network latency, database engine, and hardware.

```xml
SSIS Package with equivalent functionality processed truncate and bulk copy of 12151 rows in  00:00:04.500, go-flow of below script executed in 821.5704ms.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<pipeline>
    <variables>
        <!-- Connection strings (Replace with actual SQL Server connection details if needed) -->
        <variable name="Database1ConnStr" type="string" value="sqlserver://sa:Password123!@localhost:1433?database=database1&amp;trustServerCertificate=true" />
        <variable name="Database2ConnStr" type="string" value="sqlserver://sa:Password123!@localhost:1433?database=database2&amp;trustServerCertificate=true" />

        <!-- Initial value for ProcessDate -->
    </variables>

    <databases>
        <database name="database1" connection_string="{{Database1ConnStr}}" />
        <database name="database2" connection_string="{{Database2ConnStr}}" />
    </databases>

    <scripts>
        <!-- 1. Fetch ProcessDate (today's date in format YYYY-MM-DD) and set to ProcessDate variable -->

        <script id="GO_GET_ProcessDate" language="go">
            <![CDATA[
            package main
                import (
                    "fmt"
                    "time"
                )
                func main() {
                    today := time.Now().Format("2006-01-02")
                    fmt.Println(today)
                }   
            ]]>
        </script>

        <script id="MSSQL_TRUNCATE_xfr_cross_db_objects" language="sql" db="database2">
            <![CDATA[
                    TRUNCATE TABLE [dbo].[xfr_cross_db_objects];
                    ]]>
        </script>
        <script id="MSSQL_BLK_STREAM_to_xfr_cross_db_objects" language="sql" db="database1" target_db="database2" target_table="[dbo].[xfr_cross_db_objects]" batch_size="25000" tablock="true" check_constraints="true" fire_triggers="false" keep_nulls="true">
            <![CDATA[
                    SELECT [object_servername],[object_servicename],[object_database],[object_id],[object_schema],[object_name],[object_type],[object_desc],[LoadDate] FROM [dbo].[cross_db_objects] (NOLOCK);
                 ]]>
        </script>
    </scripts>
</pipeline>
```



### Key Capabilities
* **Variable Interpolation**: Use `{{var_name}}` syntax inside script bodies to dynamically inject pipeline variables[cite: 3, 4].
* **Output Capture**: Define the `output_var` attribute to save standard output/error into a pipeline variable for downstream consumption by SQL or Go steps[cite: 2, 4].

### XML Examples

```xml
<pipeline>
    <variables>
        <variable name="export_dir" value="C:\exports" />
    </variables>
    <scripts>
        <!-- Run external executable and store output in variable -->
        <script id="ExtractData" language="shell" output_var="GCLOUD_BILLING_JSON">
            ..\bqBilling.exe
        </script>

        <!-- PowerShell execution with variable interpolation -->
        <script id="PrepFolder" language="powershell">
            New-Item -ItemType Directory -Force -Path "{{export_dir}}"
        </script>

        <!-- Bash execution -->
        <script id="ArchiveLogs" language="bash" output_var="ARCHIVE_LOG">
            tar -czvf {{export_dir}}/archive.tar.gz {{export_dir}}/*.csv
        </script>
    </scripts>
</pipeline>
```

---

## Best Practices & Temporary Tables

In SQL Server, **local temporary tables** (prefixed with `#`, e.g., `#TempTable`) are bound to the specific database connection and session that created them. 

> [!WARNING]
> **Why cross-script `#temp` tables fail:**
> 
> Each `<script>` block in the XML pipeline retrieves an active connection from the underlying Go `database/sql` connection pool. Once that `<script>` block finishes executing, the connection is released back to the pool, automatically dropping any local `#temp` tables created in that step. Subsequent steps will fail to access them.

### Recommended Workarounds:
1. **Physical/Global Temporary Tables**: Target physical staging tables (such as standard schema tables `dbo.StagingCopy` or global temp tables `##GlobalTempTable`) when passing data across separate XML script blocks.
2. **Unified Go Sessions**: Create the `#temp` table, populate it, and execute `db.StreamETL` within a **single** interpreted `<script language="go">` block to ensure operations run in the same session context.

---

## Standard JSON Output Format

When the engine finishes executing, it outputs a clean, machine-readable JSON array to stdout, detailing the success, failure, return codes, and logs of each script block.

```json
[
  {
    "script_id": "setup_target",
    "return_code": 0,
    "results_string": "(0 row(s) returned)\n",
    "duration": "12.34ms"
  },
  {
    "script_id": "stream_sql",
    "return_code": 0,
    "results_string": "Streamed 4 row(s) directly to analytics_db.dbo.DirectStreamAudit\n",
    "duration": "45.67ms"
  }
]
```

* **`script_id`**: Corresponding XML script attribute identifier.
* **`return_code`**: `0` on success, or a descriptive error string if the script failed.
* **`results_string`**: Raw output, execution metrics, or stdout logs returned by the language engine.

---

## License
This pipeline execution engine is released under the [MIT License](LICENSE).