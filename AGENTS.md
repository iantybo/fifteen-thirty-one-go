# Agent notes (repo conventions)

# CodeRabbit post-change reviews
When you are done making changes, run CodeRabbit CLI which is located below until there are no findings. If multiple files have been changed in the repo under different directories, use the --dir flag and split out a subagent for each dir and then run coderabbit.

```
/Users/ianbowers/.local/bin/coderabbit
```

Apply the results, make any additional changes, and re-run until there are no findings.

## Go toolchain

This repo uses **Go via `goenv`**. Please ensure your active Go version is **Go 1.25.x** (we currently target **Go 1.25.0** with toolchain **go1.25.5** per `backend/go.mod`).

- **Check**: `go version`
- **Set via goenv** (example):

```bash
goenv install 1.25.5
goenv local 1.25.5
go version
```


