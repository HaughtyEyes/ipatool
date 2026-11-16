# Maintained device-identity branch

`device-identity` is the long-lived integration branch for upstream
`majd/ipatool` plus the per-account device identity override. It started from
`v2.8.1` (`3e84853e2e5d60540a5022b64895d496910144eb`) and now includes upstream
`main` through `b352566155a667d8aaeabaa7ecec4966080f92ab`. The fork's `main` and
`master` track upstream `main` without fork-specific changes. Production pins a
tested commit from `device-identity`; a moving branch is not a deployment pin.

The only code change adds the persistent `--device-mac-address` flag and the
`IPATOOL_DEVICE_MAC_ADDRESS` environment default. An explicit flag takes
precedence, including an empty flag requesting automatic detection. Invalid
nonempty values fail closed. Existing MAC validation, GUID derivation,
authentication, provisioning, purchase and download behavior are retained.

Keep each account's persisted device MAC, isolated HOME and shared framework
cache when updating this binary. Do not rotate identities or clear saved state
as part of a build/deployment. The consuming service pins this branch's exact
commit and must not apply the device identity patch a second time.

Build and verify with Go 1.26.5:

```sh
go generate ./...
go test ./...
go build -ldflags='-X github.com/majd/ipatool/v2/cmd.version=2.8.1-device-identity' -o ipatool .
```

Production builds append the fork commit's short hash to the version. Tests for
the override and CLI/environment precedence live in `cmd/device_identity_test.go`
and `pkg/util/machine/device_identity_test.go`. Build/behavior fixes for this
branch should be committed to `HaughtyEyes/ipatool` first, then consumed through
a new pinned commit in the scraper image.

## Integrating upstream updates

Use a remote named `upstream` for `https://github.com/majd/ipatool.git` and
`origin` for `https://github.com/HaughtyEyes/ipatool.git`:

```sh
git fetch upstream --tags
git switch main
git merge --ff-only upstream/main
git push origin main
git push origin upstream/main:master
git switch device-identity
git merge --no-ff upstream/main
go generate ./...
go test ./...
go build ./...
git push origin device-identity
```

Keep the merge history and existing fork commits; do not reset this branch to
upstream or force-push it. Resolve conflicts against the current upstream
interfaces, retain the optional override and automatic-detection behavior,
and adapt its tests. Keep `github.com/majd/ipatool/v2` as the module path so
upstream imports and consumers remain compatible. Do not carry old fixes that
upstream already implements.

The Device Identity CI workflow runs on pushes and pull requests to this branch,
tests the Go code and publishes a Linux amd64 binary archive and checksum as
workflow artifacts. It does not deploy production. After CI and the scraper's
CLI tests pass, update its repository/commit/version pins, build locally and
verify an actual IPA download with the existing account identity/session.
Retain the previous image and account/cache state for rollback. Local ops
documentation must record the upstream merge, exact fork commit and acceptance.
