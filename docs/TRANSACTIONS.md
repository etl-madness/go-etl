# Database Transactions in Flow Pipelines

Flow supports wrapping sequential SQL script execution inside an atomic database transaction. This ensures that a group of SQL scripts either succeeds together or rolls back completely if an error occurs.

---

## 1. Configuration Syntax

Transactions are declared at the `<group>` node level using two attributes:
*   `transaction="true"`: Enables transaction wrapping for all SQL script nodes inside the group.
*   `db="..."` (or `database="..."`): Specifies the target database handle to run the transaction on.

### Basic Example
```xml
<pipeline>
    <databases>
        <database name="sales_db" driver="postgres" connection_string="postgresql://..." />
    </databases>
    <scripts>
        <group id="update_sales_txn" transaction="true" db="sales_db">
            <script id="deduct_inventory" language="sql" db="sales_db">
                UPDATE inventory SET stock = stock - 1 WHERE item_id = 42;
            </script>
            <script id="record_sale" language="sql" db="sales_db">
                INSERT INTO sales (item_id, qty) VALUES (42, 1);
            </script>
        </group>
    </scripts>
</pipeline>
```

---

## 2. Dynamic Loops & Nested Transactions (`<foreach>`)

If a group with `transaction="true"` is placed inside a `<foreach>` loop, Flow begins, executes, and commits/rolls back a new transaction **per loop iteration**. 

```xml
<pipeline>
    <scripts>
        <!-- Loop driver gets item IDs -->
        <foreach id="process_items" language="sql" db="sales_db">
            SELECT item_id, price FROM active_promotions;
            
            <!-- A transaction is started and finished for each promotion processed -->
            <group id="apply_promo_txn" transaction="true" db="sales_db">
                <script id="update_price" language="sql" db="sales_db">
                    UPDATE products SET price = {{price}} WHERE id = {{item_id}};
                </script>
                <script id="log_history" language="sql" db="sales_db">
                    INSERT INTO pricing_log (product_id, new_price) VALUES ({{item_id}}, {{price}});
                </script>
            </group>
        </foreach>
    </scripts>
</pipeline>
```

---

## 3. Parallel Execution & Isolation (`<parallel>`)

When running concurrent tasks in a `<parallel>` block:
*   Flow isolates each parallel branch inside its own execution context with a unique cloned executor.
*   If a `<group transaction="true">` is placed inside a parallel branch, its active transaction is completely isolated to that branch's worker connection.
*   This prevents race conditions or shared transaction states across parallel threads.

```xml
<pipeline>
    <scripts>
        <parallel max_threads="2">
            <!-- Branch 1: isolated txn on sales_db -->
            <group transaction="true" db="sales_db">
                <script language="sql" db="sales_db">INSERT INTO logs VALUES ('Thread A started');</script>
                <script language="sql" db="sales_db">UPDATE stats SET count = count + 1;</script>
            </group>

            <!-- Branch 2: isolated txn on sales_db -->
            <group transaction="true" db="sales_db">
                <script language="sql" db="sales_db">INSERT INTO logs VALUES ('Thread B started');</script>
                <script language="sql" db="sales_db">UPDATE stats SET count = count + 1;</script>
            </group>
        </parallel>
    </scripts>
</pipeline>
```

---

## 4. Interaction with SQL Server Bulk Copy (`StreamETL`)

Flow has high-performance support for Microsoft SQL Server native TDS bulk streaming copy via the `StreamETL` function (`<script target_table="...">` tag). 

### How Bulk Copy Works
The MS SQL Server bulk copy utility loads data efficiently by writing data pages directly to the database without the overhead of standard row-by-row transaction logs. It utilizes the `mssql.CopyIn` interface.

### Bulk Copy Batch-Level Transactions
Unlike standard sequential SQL scripts, **SQL Server bulk copy manages its own internal transactions** to flush rows in high-speed chunks (defined by `batch_size`, which defaults to `10000` rows). Each batch flush:
1. Opens a separate, native bulk transaction.
2. Writes the chunk of rows.
3. Commits that batch chunk immediately to disk.

### Impact of Mixing Group Transactions and Bulk Copy
> [!WARNING]
> While a standard `<group transaction="true">` is excellent for atomic transactional integrity on conventional queries, you should avoid placing high-throughput bulk copy stream scripts (`<script target_table="...">`) inside a transaction-enabled group.

Here is how configuration options impact bulk copy:

1.  **Atomicity vs. Recovery**: 
    If a bulk copy script is wrapped inside an outer group transaction, standard driver behaviors might force all streamed batches to use the same underlying physical connection. However, because bulk copying relies on native SQL Server TDS fast-load streams, mixing outer group transactions with internal batch commits can cause locking, socket blocks, or driver-level command overlap errors.
2.  **TABLOCK (Minimal Logging)**:
    By default, bulk insert operations use the `tablock` option to lock the target table during inserts. This minimizes transactional logging and maximizes throughput. However, if a table lock is acquired within a long-running standard transaction, concurrent queries targeting that table are fully blocked until the entire transaction group is either committed or rolled back.
3.  **Check Constraints & Triggers**:
    By default, constraints checking (`check_constraints`) and triggers (`fire_triggers`) are turned off to speed up ingestion. Enabling them forces SQL Server to run validations, which increases logging overhead and log-write lock contention. Under an outer group transaction, this logging overhead can lead to rapid database transaction log depletion.

### Summary Best Practice
*   **Use `<group transaction="true">`** for transactional business logic, updates, inserts, and system state steps.
*   **Do NOT use outer group transactions** for high-volume data streams (`StreamETL`). Instead, let the streaming engine handle batch-level transaction commits naturally.
