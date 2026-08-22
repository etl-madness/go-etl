# Pipeline Specification & Flowchart .\examples\go_logfile_example.xml

## Execution Flow Diagram

```mermaid
flowchart TD
    Start([Start Pipeline])
    is_data_uptodate["is_data_uptodate<br/>(SQL)"]
    Start --> is_data_uptodate
    if_cond_1{"❓ If: DATA_UPTODATE == false"}
    if_end_2(( Rejoin ))
    parse_jil_command["parse_jil_command<br/>(GO)"]
    find_log_in_file["find_log_in_file<br/>(GO)"]
    parse_jil_command --> find_log_in_file
    read_log_content["read_log_content<br/>(GO)"]
    find_log_in_file --> read_log_content
    if_cond_1 -- "Yes / Then" --> parse_jil_command
    read_log_content --> if_end_2
    data_uptodate_message["data_uptodate_message<br/>(GO)"]
    if_cond_1 -- "No / Else" --> data_uptodate_message
    data_uptodate_message --> if_end_2
    is_data_uptodate --> if_cond_1
    if_end_2 --> End_3([End Pipeline])
```

## Configured Variables

| Name | Type | Default Value |
|---|---|---|
| **JilPath** | `string` | `./examples/JIL/csx_logfile_example.jil` |
| **Database1ConnStr** | `string` | `sqlserver://sa:Password123!@localhost:1433?database=database1&trustServerCertificate=true` |

## Configured Databases

| Alias Name | Connection String / Variable Reference |
|---|---|
| **database1** | `{{Database1ConnStr}}` |

## SCRIPTS

| Language | ID/Name | XPath Location | Source Database | Target Database | Target Table | Batch Size | Value |
|---|---|---|---|---|---|---|---|
| **sql** | **is_data_uptodate** | `/Q{}pipeline[1]/Q{}scripts[1]/Q{}script[1]` | `database1` | `database1` | `` | `` | <code>SELECT CASE<br/>WHEN EXISTS (<br/>SELECT 1<br/>FROM [PROTO].[dbo].[GcpBillingExport]<br/>WHERE CONVERT(DATE, ExportTime) = CONVERT(DATE, GETUTCDATE())<br/>) THEN 'true'<br/>ELSE 'false'<br/>END AS [DATA_UPTODATE];</code> |
| **go** | **parse_jil_command** | `/Q{}pipeline[1]/Q{}scripts[1]/Q{}if[1]/Q{}then[1]/Q{}script[1]` | `` | `` | `` | `` | <code>package main<br/>import (<br/>"bufio"<br/>"fmt"<br/>"os"<br/>"regexp"<br/>"strings"<br/>"host/vars/vars"<br/>)<br/>func main() {<br/>jilPath := vars.GetString("JilPath")<br/>if jilPath == "" {<br/>jilPath = "job.jil"<br/>}<br/>file, err := os.Open(jilPath)<br/>if err != nil {<br/>fmt.Printf("Error: File '%s' not found.", jilPath)<br/>return<br/>}<br/>defer file.Close()<br/>re := regexp.MustCompile(`(?i)^\s*command:\s*"?([^"\r\n]+)"?\s*$`)<br/>scanner := bufio.NewScanner(file)<br/>var commandPath string<br/>for scanner.Scan() {<br/>line := scanner.Text()<br/>matches := re.FindStringSubmatch(line)<br/>if len(matches) > 1 {<br/>commandPath = strings.Trim(strings.TrimSpace(matches[1]), `"`)<br/>break<br/>}<br/>}<br/>if commandPath != "" {<br/>fmt.Print(commandPath)<br/>} else {<br/>fmt.Print("Error: 'command:' definition not found in JIL file.")<br/>}<br/>}</code> |
| **go** | **find_log_in_file** | `/Q{}pipeline[1]/Q{}scripts[1]/Q{}if[1]/Q{}then[1]/Q{}script[2]` | `` | `` | `` | `` | <code>package main<br/>import (<br/>"bufio"<br/>"fmt"<br/>"os"<br/>"regexp"<br/>"strings"<br/>"host/vars/vars"<br/>)<br/>func main() {<br/>targetPath := vars.GetString("FilePath")<br/>if targetPath == "" {<br/>targetPath = "setup.bat"<br/>}<br/>file, err := os.Open(targetPath)<br/>if err != nil {<br/>fmt.Printf("Error: File '%s' not found.", targetPath)<br/>return<br/>}<br/>defer file.Close()<br/>re := regexp.MustCompile(`(?i)^\s*SET\s+"?\s*LOG_FILE\s*=\s*"?(.*?)"?\s*$`)<br/>scanner := bufio.NewScanner(file)<br/>var logFile string<br/>for scanner.Scan() {<br/>line := scanner.Text()<br/>matches := re.FindStringSubmatch(line)<br/>if len(matches) > 1 {<br/>logFile = strings.Trim(strings.TrimSpace(matches[1]), `"`)<br/>break<br/>}<br/>}<br/>if logFile != "" {<br/>fmt.Print(logFile)<br/>} else {<br/>fmt.Print("Error: 'SET LOG_FILE' line not found in file.")<br/>}<br/>}</code> |
| **go** | **read_log_content** | `/Q{}pipeline[1]/Q{}scripts[1]/Q{}if[1]/Q{}then[1]/Q{}script[3]` | `` | `` | `` | `` | <code>package main<br/>import (<br/>"bytes"<br/>"encoding/json"<br/>"fmt"<br/>"os"<br/>"path/filepath"<br/>"strings"<br/>"time"<br/>"host/vars/vars"<br/>)<br/>type Response struct {<br/>Content   string `json:"content"`<br/>Timestamp string `json:"timestamp"`<br/>Status    string `json:"status"`<br/>}<br/>func main() {<br/>batPath := vars.GetString("FilePath")<br/>rawLogPath := vars.GetString("LOGFILE_RESULTS")<br/>if strings.TrimSpace(rawLogPath) == "" {<br/>fmt.Println("Error: 'LOGFILE_RESULTS' is empty.")<br/>return<br/>}<br/>cleanLogPath := filepath.FromSlash(strings.ReplaceAll(rawLogPath, "\\", "/"))<br/>resolvedPath := cleanLogPath<br/>if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {<br/>if batPath != "" {<br/>batDir := filepath.Dir(batPath)<br/>fallback := filepath.Join(batDir, cleanLogPath)<br/>if _, err := os.Stat(fallback); err == nil {<br/>resolvedPath = fallback<br/>}<br/>}<br/>}<br/>contentBytes, err := os.ReadFile(resolvedPath)<br/>if err != nil {<br/>fmt.Printf("Error: Log file not found at '%s'\n", resolvedPath)<br/>return<br/>}<br/>// 1. Convert CRLF (\r\n) to LF (\n) to clean up line returns<br/>cleanContent := strings.ReplaceAll(string(contentBytes), "\r\n", "\n")<br/>cleanContent = strings.ReplaceAll(cleanContent, "\r", "")<br/>res := Response{<br/>Content:   cleanContent,<br/>Timestamp: time.Now().UTC().Format(time.RFC3339),<br/>Status:    "Success",<br/>}<br/>// 2. Disable HTML escaping so single quotes (') don't become \u0027<br/>var buf bytes.Buffer<br/>enc := json.NewEncoder(&buf)<br/>enc.SetEscapeHTML(false)<br/>enc.SetIndent("", "  ")<br/>if err := enc.Encode(res); err != nil {<br/>fmt.Printf("Error encoding JSON: %v\n", err)<br/>return<br/>}<br/>fmt.Print(strings.TrimRight(buf.String(), "\n"))<br/>}</code> |
| **go** | **data_uptodate_message** | `/Q{}pipeline[1]/Q{}scripts[1]/Q{}if[1]/Q{}else[1]/Q{}script[1]` | `` | `` | `` | `` | <code>package main<br/>import (<br/>"fmt"<br/>)<br/>func main() {<br/>fmt.Println("Data is up-to-date. No further action required.")<br/>}</code> |

## Results
Pipeline Start Time: 2026-08-22 14:12:14.813
| Script ID | Return Code | Results |
| :--- | :--- | :--- |
| is_data_uptodate | 0 | DATA_UPTODATE<br>false<br><br>(1 row(s) returned)<br> |
| parse_jil_command | 0 | examples\BAT\csx_logfile_example.bat |
| find_log_in_file | 0 | examples\LOGS\csx_logfile_example.txt |
| read_log_content | 0 | {<br>  "content": "You must install or update .NET to run this application.\n\nApp: C:\\Tools\\dotnet-script.dll\nArchitecture: x64\nFramework: 'Microsoft.NETCore.App', version '8.0.0' (x64)\n.NET location: C:\\Program Files\\dotnet\\\n\nThe following frameworks were found:\n  10.0.0 at [C:\\Program Files\\dotnet\\shared\\Microsoft.NETCore.App]\n\nLearn about framework resolution:\nhttps://aka.ms/dotnet/missing-app-runtime",<br>  "timestamp": "2026-08-22T18:12:14Z",<br>  "status": "Success"<br>} |
Pipeline End Time:   2026-08-22 14:12:14.865
Pipeline Duration:   51.7472ms
