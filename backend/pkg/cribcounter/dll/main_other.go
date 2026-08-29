//go:build !windows

// Command cribcounterdll builds the cribbage counter as a Windows DLL (see
// main_windows.go). On non-Windows platforms it compiles to a no-op so that
// `go build ./...` / `go test ./...` succeed everywhere (e.g. in CI on Linux).
package main

func main() {}
