# Executing Database Operations from Go Scripts (`GO_TO_SQL.md`)

The `flow` pipeline engine allows developers to write and execute embedded Go scripts that interact with databases natively [cite: 1, 3]. The host engine exposes registered database handles and optimized ETL stream utilities to Go scripts via the virtual `host/db/db` package [cite: 3].

When your pipeline needs to perform operations that are too complex for standalone SQL statements—such as executing HTTP API lookups per row, running external binary commands, building dynamic query logic, or orchestrating custom transactions—you can retrieve rows and write data directly using these methods [cite: 1, 3].

---

## Method 1: Manual Row Query & Transaction Inserts (`db.Get`)

### Description
This method uses standard `database/sql` patterns [cite: 2, 5]. It retrieves the raw connection pool handles (`*sql.DB`) for both the source and target databases [cite: 2, 5]. You execute a query on the source database, iterate over the returned rows manually, and insert them into the target database using prepared parameter statements wrapped inside a database transaction [cite: 2, 3].

### When to use
* **Row-by-Row Logic:** You need to mutate, format, or calculate data for individual rows before inserting them into the target database [cite: 2, 3].
* **External Lookups:** You need to enrich data by calling a REST API or performing supplementary logic during the iteration process [cite: 1, 3].
* **Upserts / Complex Commands:** You need to run logic more complex than a standard bulk `INSERT` (e.g., executing `UPDATE` or `DELETE` statements conditionally) [cite: 3, 4].

### Example
```xml
<script id="ManualRowTransfer" language="go">
package main

import (
    "fmt"
    "host/db/db"
)

func main() {
    // 1. Retrieve *sql.DB handles from the pipeline registry
    srcDB, err := db.Get("source_db")
    if err != nil {
        panic(err)
    }
    dstDB, err := db.Get("target_db")
    if err != nil {
        panic(err)
    }

    // 2. Query source rows
    rows, err := srcDB.Query("SELECT user_id, email, status FROM active_users")
    if err != nil {
        panic(err)
    }
    defer rows.Close()

    // 3. Begin transaction on destination database
    tx, err := dstDB.Begin()
    if err != nil {
        panic(err)
    }
    defer tx.Rollback()

    stmt, err := tx.Prepare("INSERT INTO user_summary (user_id, email, status) VALUES (?, ?, ?)")
    if err != nil {
        panic(err)
    }
    defer stmt.Close()

    // 4. Scan source rows and insert into target
    rowCount := 0
    for rows.Next() {
        var userID int
        var email, status string

        if err := rows.Scan(&userID, &email, &status); err != nil {
            panic(err)
        }

        // Apply custom logic, validation, or API calls here before execution
        if _, err := stmt.Exec(userID, email, status); err != nil {
            panic(err)
        }
        rowCount++
    }

    // 5. Commit transaction
    if err := tx.Commit(); err != nil {
        panic(err)
    }

    fmt.Printf("Successfully processed and inserted %d rows.\n", rowCount)
}
</script>
```

---

## Method 2: High-Performance Bulk Stream ETL (`db.StreamETL`)

### Description
This method delegates the heavy lifting back to the `flow` pipeline's optimized streaming engine [cite: 1, 2]. By calling `db.StreamETL`, the engine manages the background routines, streaming data from the query results of the source database directly into the destination table [cite: 2, 3]. It automatically formats placeholder syntax across heterogeneous database drivers and handles batched inserts [cite: 2, 4].

### When to use
* **Bulk Dataset Transfers:** You are simply moving large amounts of records (e.g., thousands or millions) directly from one database to another without modifying the row contents [cite: 2, 4].
* **Cross-Driver Copying:** You are migrating data between different engine types (e.g., PostgreSQL to MSSQL) and want `flow` to handle the batch limits and parameter formats automatically [cite: 2, 4].
* **MSSQL Target Optimization:** You are copying data to SQL Server, which will benefit directly from the native TDS Bulk Copy (`mssql.CopyIn`) routines handled under the hood [cite: 2, 4].
* **Dynamic Stream Targets:** You need to construct the driver query, the target table name, or the batch size dynamically from environment variables or external binary execution outputs retrieved at runtime [cite: 3, 23].

### Example 1: Basic Bulk Stream
```xml
<script id="EngineStreamETL" language="go">
package main

import (
    "fmt"
    "host/db/db"
)

func main() {
    // Configure batch and execution tuning flags
    opts := db.ETLOptions{
        BatchSize:        10000, // Number of rows per batch flush
        Tablock:          true,  // Acquire table lock for minimal logging on SQL Server targets
        CheckConstraints: false,
    }

    // Execute bulk stream from source DB query directly to destination target table
    rowsCopied, err := db.StreamETL(
        "source_db",                                          // Registered source DB name
        "SELECT id, product_code, price FROM warehouse_stock", // Source SQL query
        "target_db",                                          // Registered destination DB name
        "inventory_copy",                                     // Target table name
        opts,
    )
    if err != nil {
        panic(fmt.Sprintf("StreamETL failed: %v", err))
    }

    fmt.Printf("Successfully streamed %d rows directly to inventory_copy\n", rowsCopied)
}
</script>
```

### Example 2: Stream ETL Configured via Execution Variables
This example demonstrates combining the `host/vars/vars` package with `db.StreamETL` to inject dynamically evaluated pipeline parameters directly into the stream config [cite: 3].

```xml
<pipeline>
    <variables>
        <variable name="SourceDB" value="prod_db" />
        <variable name="TargetDB" value="warehouse_db" />
        <variable name="DynamicTargetTable" value="monthly_sales_summary" />
        <variable name="TargetBatchSize" type="int" value="25000" />
        <variable name="MinDateFilter" value="2023-01-01" />
    </variables>
    <scripts>
        <script id="DynamicStreamETL" language="go">
        package main

        import (
            "fmt"
            "host/db/db"
            "host/vars/vars"
        )

        func main() {
            // Retrieve dynamic configuration from the pipeline execution environment
            srcDB := vars.GetString("SourceDB")
            dstDB := vars.GetString("TargetDB")
            targetTable := vars.GetString("DynamicTargetTable")
            batchLimit := vars.GetInt("TargetBatchSize")
            minDate := vars.GetString("MinDateFilter")

            // Construct dynamic SQL string using the retrieved variable
            driverQuery := fmt.Sprintf("SELECT order_id, product_id, total FROM sales WHERE order_date >= '%s'", minDate)

            // Configure batch settings dynamically
            opts := db.ETLOptions{
                BatchSize:        batchLimit,
                Tablock:          true,
                CheckConstraints: false,
            }

            // Execute the bulk stream with the dynamic parameters
            rowsCopied, err := db.StreamETL(
                srcDB,
                driverQuery,
                dstDB,
                targetTable,
                opts,
            )
            if err != nil {
                panic(fmt.Sprintf("Dynamic StreamETL failed: %v", err))
            }

            fmt.Printf("Dynamically streamed %d rows from %s to %s.%s\n", rowsCopied, srcDB, dstDB, targetTable)
        }
        </script>
    </scripts>
</pipeline>
```

---

## Method 3: Running External Commands (`bqBilling`) & Streaming via `db.StreamETL`

### Description
In this workflow, an embedded Go script retrieves environment/pipeline variables using `host/vars/vars` [cite: 3], sets up operating system environment variables required by an external CLI utility like `bqBilling` [cite: 23], executes `bqBilling` using `os/exec` [cite: 3, 23], parses its JSON or text output [cite: 23], and uses the parsed information (such as target billing table paths or dynamic query criteria) to trigger `db.StreamETL` [cite: 2, 3].

### `bqBilling` Binary Context
The `bqBilling` tool requires three environment variables to execute [cite: 23]:
1. `GCP_PROJECT_ID`: GCP Project ID [cite: 23].
2. `BQ_BILLING_TABLE`: The BigQuery billing export table path (formatted as `project_id.dataset.table_name`) [cite: 23].
3. `GOOGLE_APPLICATION_CREDENTIALS`: Path to the service account JSON key file [cite: 23].

It queries the specified BigQuery table and outputs formatted JSON records containing billing metadata (`billing_account_id`, `service`, `sku`, `project`, `cost`, `currency`, `usage`, etc.) to standard output [cite: 23].

### Example: Running `bqBilling` and Executing `db.StreamETL`

```xml
<pipeline>
    <variables>
        <variable name="GCP_PROJECT_ID" value="my-gcp-billing-project" />
        <variable name="BQ_BILLING_TABLE" value="my-gcp-billing-project.billing_ds.gcp_billing_export_v1" />
        <variable name="GOOGLE_APPLICATION_CREDENTIALS" value="/etc/gcp/credentials.json" />
        <variable name="SourceDB" value="bq_source_conn" />
        <variable name="TargetDB" value="postgres_dw" />
        <variable name="TargetTable" value="gcp_billing_records" />
        <variable name="BatchSize" type="int" value="5000" />
    </variables>
    <scripts>
        <script id="RunBQBillingAndStream" language="go">
        package main

        import (
            "encoding/json"
            "fmt"
            "os"
            "os/exec"
            "strings"
            "host/db/db"
            "host/vars/vars"
        )

        // BillingSummary represents structured output parsed from bqBilling output
        type BillingSummary struct {
            BillingAccountID string  `json:"billing_account_id"`
            Cost             float64 `json:"cost"`
            Currency         string  `json:"currency"`
        }

        func main() {
            // 1. Extract execution variables from registry
            projectID := vars.GetString("GCP_PROJECT_ID")
            bqTable := vars.GetString("BQ_BILLING_TABLE")
            credsPath := vars.GetString("GOOGLE_APPLICATION_CREDENTIALS")
            srcDB := vars.GetString("SourceDB")
            dstDB := vars.GetString("TargetDB")
            targetTable := vars.GetString("TargetTable")
            batchSize := vars.GetInt("BatchSize")

            // 2. Set OS Environment variables required by bqBilling binary
            os.Setenv("GCP_PROJECT_ID", projectID)
            os.Setenv("BQ_BILLING_TABLE", bqTable)
            os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsPath)

            // 3. Execute bqBilling CLI command
            cmd := exec.Command("bqBilling")
            out, err := cmd.CombinedOutput()
            if err != nil {
                panic(fmt.Sprintf("bqBilling execution failed: %v, output: %s", err, string(out)))
            }

            fmt.Printf("bqBilling executed successfully. Raw Output Length: %d bytes\n", len(out))

            // 4. Parse bqBilling output to build dynamic query parameters
            var billingAccountID string
            lines := strings.Split(string(out), "\n")
            for _, line := range lines {
                if strings.Contains(line, `"billing_account_id"`) {
                    var summary BillingSummary
                    if jsonErr := json.Unmarshal([]byte(line), &summary); jsonErr == nil && summary.BillingAccountID != "" {
                        billingAccountID = summary.BillingAccountID
                        break
                    }
                }
            }

            // 5. Construct the dynamic extraction query for StreamETL
            var driverQuery string
            if billingAccountID != "" {
                driverQuery = fmt.Sprintf("SELECT * FROM `%s` WHERE billing_account_id = '%s'", bqTable, billingAccountID)
            } else {
                driverQuery = fmt.Sprintf("SELECT * FROM `%s`", bqTable)
            }

            // 6. Execute StreamETL to insert results into target table
            opts := db.ETLOptions{
                BatchSize:        batchSize,
                Tablock:          true,
                CheckConstraints: false,
            }

            rowsInserted, err := db.StreamETL(srcDB, driverQuery, dstDB, targetTable, opts)
            if err != nil {
                panic(fmt.Sprintf("StreamETL failed inserting bqBilling records: %v", err))
            }

            fmt.Printf("Successfully processed bqBilling output and streamed %d records into %s.%s\n", 
                rowsInserted, dstDB, targetTable)
        }
        </script>
    </scripts>
</pipeline>
```
