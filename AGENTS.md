# Agent notes (repo conventions)

## Go toolchain

This repo uses **Go via `goenv`**. Please ensure your active Go version is **Go 1.25.x** (we currently target **Go 1.25.0** with toolchain **go1.25.5** per `backend/go.mod`).

- **Check**: `go version`
- **Set via goenv** (example):

```bash
goenv install 1.25.5
goenv local 1.25.5
go version
```


