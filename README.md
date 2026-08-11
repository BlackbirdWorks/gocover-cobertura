# gocover-cobertura

[![Build Status](https://github.com/BlackbirdWorks/gocover-cobertura/actions/workflows/ci.yml/badge.svg)](https://github.com/BlackbirdWorks/gocover-cobertura/actions/workflows/ci.yml)

Convert Go cover profiles to [Cobertura](http://cobertura.sourceforge.net/) XML format.
This is useful for generating coverage reports in CI pipelines (like Jenkins, GitLab CI, GitHub Actions) from `go test -coverprofile` output.

## Installation

Install the latest version using `go install`:

```bash
go install github.com/blackbirdworks/gocover-cobertura@latest
```

## Usage

`gocover-cobertura` supports reading from stdin or reading directly from files, including glob patterns.

```bash
Usage: gocover-cobertura [flags]

Convert Go cover profile to Cobertura XML format

Flags:
  -h, --help          Show context-sensitive help.
  -f, --file="-"      Input coverage file path (default '-' for stdin)
  -p, --pattern=""    Glob pattern for matching files (e.g. '**/*.out')
  -o, --output="-"    Output Cobertura XML file path (default '-' for stdout)
```

### Examples

**Standard Unix pipes:**
```bash
go test -coverprofile=coverage.out ./...
gocover-cobertura < coverage.out > coverage.xml
```

**Using the `--file` flag:**
```bash
gocover-cobertura -f coverage.out -o coverage.xml
```

**Using glob patterns for multiple coverage files:**
```bash
gocover-cobertura --pattern="tests/**/*.out" -o coverage.xml
```

## Authors

* [BlackbirdWorks](https://github.com/BlackbirdWorks) (Current Maintainers)
* [Yukinari Toyota (t-yuki)](https://github.com/t-yuki) (Original Author)

## Thanks

This tool originated from [gocov-xml](https://github.com/AlekSi/gocov-xml) by [Alexey Palazhchenko (AlekSi)](https://github.com/AlekSi).
