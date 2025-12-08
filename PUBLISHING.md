# Publishing LogDot Go SDK

This guide covers publishing the LogDot SDK as a Go module to pkg.go.dev.

## How Go Module Publishing Works

Unlike npm or PyPI, Go modules don't have a central upload process. Instead:

1. You host your code on a public repository (GitHub, GitLab, etc.)
2. You create version tags following semantic versioning
3. The Go module proxy automatically indexes your module
4. Users import your module using the module path

## Prerequisites

1. **Public Repository**: Your code must be in a public Git repository
2. **Module Path**: Must match repository URL (e.g., `github.com/logdot-io/logdot-go`)
3. **Git Tags**: Use semantic version tags (e.g., `v1.0.0`)

## Pre-Publish Checklist

- [ ] Verify `go.mod` has correct module path
- [ ] Ensure all code compiles: `go build ./...`
- [ ] Run tests: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Update `README.md` with correct import path
- [ ] Ensure `LICENSE` file exists

## Verify Module Configuration

Check `go.mod`:

```go
module github.com/logdot-io/logdot-go

go 1.21
```

The module path must exactly match your repository URL.

## Version Tagging

### Creating a Release

```bash
# Ensure you're on main branch with latest changes
git checkout main
git pull origin main

# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to remote
git push origin v1.0.0
```

### Semantic Versioning

- **v1.0.0**: Initial stable release
- **v1.0.1**: Patch release (bug fixes)
- **v1.1.0**: Minor release (new features, backwards compatible)
- **v2.0.0**: Major release (breaking changes)

**Important**: For v2+, you must update the module path:
```go
module github.com/logdot-io/logdot-go/v2
```

## Triggering pkg.go.dev Indexing

After pushing a tag, pkg.go.dev will automatically index your module. To speed this up:

### Option 1: Request via proxy

```bash
GOPROXY=proxy.golang.org go list -m github.com/logdot-io/logdot-go@v1.0.0
```

### Option 2: Visit the package page

Go to `https://pkg.go.dev/github.com/logdot-io/logdot-go` - this triggers indexing.

### Option 3: Import in any Go project

```bash
go get github.com/logdot-io/logdot-go@v1.0.0
```

## Verifying Publication

1. Visit [pkg.go.dev/github.com/logdot-io/logdot-go](https://pkg.go.dev/github.com/logdot-io/logdot-go)
2. Check that your version appears in the versions list
3. Verify documentation is rendered correctly

## Users Installing Your Module

Users can install with:

```bash
go get github.com/logdot-io/logdot-go@latest

# Or a specific version
go get github.com/logdot-io/logdot-go@v1.0.0
```

## Documentation

pkg.go.dev automatically generates documentation from:

- Package comments (comments before `package` declaration)
- Exported function/type comments
- `README.md` file
- `doc.go` file (if present)

### Example doc.go

```go
// Package logdot provides a client for the LogDot cloud logging service.
//
// Basic usage:
//
//     client := logdot.New("your-api-key", logdot.WithHostname("my-app"))
//     client.Info(ctx, "Application started", nil)
//
// See https://logdot.io for more information.
package logdot
```

## Retracting Versions

If you need to mark a version as broken:

1. Add a `retract` directive to `go.mod`:

```go
module github.com/logdot-io/logdot-go

go 1.21

retract (
    v1.0.1 // Contains critical bug in logging
)
```

2. Tag and release a new version with this change

## Private Modules

For private repositories, users need to configure:

```bash
# Set GOPRIVATE
go env -w GOPRIVATE=github.com/logdot/*

# Or use .netrc for authentication
echo "machine github.com login USERNAME password TOKEN" >> ~/.netrc
```

## CI/CD (Optional)

Example GitHub Actions workflow for testing on release:

```yaml
name: Release
on:
  push:
    tags:
      - 'v*'
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - run: go test ./...
      - run: go vet ./...

  trigger-proxy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - name: Trigger module proxy
        run: |
          curl "https://proxy.golang.org/github.com/logdot-io/logdot-go/@v/${{ github.ref_name }}.info"
```

## Troubleshooting

### "module not found"
- Ensure repository is public
- Check module path matches repository URL exactly
- Wait a few minutes for proxy to index

### "invalid version"
- Tags must start with `v` (e.g., `v1.0.0`, not `1.0.0`)
- Must follow semantic versioning format

### Documentation not showing
- Ensure exported items have comments
- Check for syntax errors in doc comments
- Trigger re-indexing by visiting pkg.go.dev

## Resources

- [Go Module Reference](https://go.dev/ref/mod)
- [pkg.go.dev About](https://pkg.go.dev/about)
- [Semantic Versioning](https://semver.org/)
- [Publishing Go Modules](https://go.dev/blog/publishing-go-modules)
