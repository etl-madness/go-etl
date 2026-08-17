# **Yaegi Go Interpreter**

 Embedded in the engine for executing interpreted Go scripts, required for `<script language="go">` blocks, may need to have the `GOPATH` environment variable set for package imports. In some cases you may need to explicitly pass the `--gopath` flag to the CLI runner if your Go environment is non-standard.  In the event that you are running code that needs special imports you will have to stage source clones of those packages into your GOPATH using the legacy src pathing, or install and run yaegi. Yaegi does not support `unsafe` package imports, so any Go code that uses `unsafe` will not run in the interpreted context. Under those conditions you will need to compile and run your Go code as a native binary, and use `<script language="go">` to execute it directly.

**Example setting up for special packages:**

```bash
mkdir -p $GOPATH/src/somepackage-a/go
git clone https://github.com/somepackage-a.git $GOPATH/src/somepackage-a/go
 
```
