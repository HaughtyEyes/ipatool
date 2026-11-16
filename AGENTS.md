# Maintained ipatool fork

- `device-identity` is the long-lived branch for production changes. Preserve
  upstream history and merge new `majd/ipatool` commits instead of resetting,
  recreating or force-pushing this branch. `main` and `master` remain upstream
  mirrors. See `FORK.md` for the update and deployment procedure.
- Keep changes small and compatible with upstream interfaces. Retain the module
  path `github.com/majd/ipatool/v2` and upstream behavior when no MAC override is
  supplied. Invalid explicit identities fail closed; explicit CLI flags take
  precedence over the environment, including the empty flag.
- Commit ipatool implementation fixes here. The scraper consumes a tested exact
  commit; do not move fixes into an untracked build-time patch there.
- Run `gofmt` on changed Go files, `go generate ./...`, `go test ./...` and
  `go build ./...`. The SAP signer integration test requires network access;
  report cached results and live endpoint failures accurately.
- Preserve account identities, isolated HOME directories and shared framework
  cache during compatibility testing. Stop on human verification challenges.
- Verify CLI compatibility and an actual IPA acquisition before claiming a
  production upgrade succeeded. Keep prior images and state for rollback.
