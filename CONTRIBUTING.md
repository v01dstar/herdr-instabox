# Contributing

## Local development

Use Linux or macOS and the Go version in `go.mod` (currently 1.27.1). Python 3 runs the dependency-free documentation checker. Herdr/cloud credentials are not needed for automated tests.

```bash
go test ./...
go test -race ./...
go vet ./...
go build -o bin/herdr-instabox .
python3 scripts/check-docs.py
```

Format changed Go files with `gofmt`. For manual UI work, build and link the checkout as described in the [README](README.md). Linking skips build hooks; install missing keys explicitly.

## Keep the important boundaries intact

- Account context is chosen before concurrency starts; never swap credentials/environment under a live UI request.
- Named profiles must not inherit personal Secrets/env/snapshot defaults.
- Authenticate and check scope before mutation or certificate issuance; filtering lists is not sufficient.
- Shared Herdr registry and SSH configuration writes require root-level serialization.
- Malformed config must not silently change credential selections.
- Do not log tokens, env values, auth URLs/device codes, private keys, or raw credential files.
- Never make tests depend on your real HOME, cloud account, or installed Herdr state.

## Tests and documentation

Use `newFake`/`fakeAccounts` and CLI subprocess fixtures for regressions. Add isolation tests for any operation that removes state or creates resources. Example JSON is loaded by the production config reader during tests; update examples and the reference whenever fields change. Run the docs checker to catch broken local links and anchors.

Changes to user-visible commands need help text, reference/recipe updates, and a changelog entry. Distinguish offline tests from live evidence. Follow the [live verification guide](docs/verification.md) before claiming a new provider flow or lifecycle behavior is verified.

## Pull requests and reports

Describe the user-visible problem, tests run, migration impact, and any unverified live behavior. For bugs, include OS, Herdr version, plugin commit, sanitized `doctor` output, and minimal reproduction steps. Use the [security policy](SECURITY.md) for sensitive reports.

This fork retains upstream attribution and the original module/plugin identifiers for compatibility. The checkout does not currently declare a repository license; do not invent a license header or assume upstream code can be relicensed. Resolve licensing with the maintainers before a licensing/redistribution change.
