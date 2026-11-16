# Production device identity branch

This branch is based on upstream `majd/ipatool` release `v2.8.1`, commit
`3e84853e2e5d60540a5022b64895d496910144eb`. The fork's `main` and `master`
branches track upstream `main`; production stays on this stable release until
an explicit upgrade is tested.

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
