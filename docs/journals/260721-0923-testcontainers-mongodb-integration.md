# Testcontainers MongoDB Integration Journal

## Context

MongoDB integration tests previously depended on contributors manually starting
a database and setting `MONGODB_TEST_URL`. The suite now provisions its own
MongoDB 8 instances when Docker is available.

## What Changed

- Added Testcontainers Go and MongoDB module v0.43.0.
- Added a shared lazy test manager that starts one `mongo:8` container per
  `storage`, `lol`, and `stats` package and terminates it after that package's
  suite.
- Preserved `MONGODB_TEST_URL` as an external-database override.
- Docker-unavailable environments emit an explicit warning and skip MongoDB
  tests; a regression test verifies both the warning and skip behavior.
- Once the Docker provider is healthy, container startup and connection-string
  failures are fatal. Cleanup failures also make an otherwise passing suite
  fail.
- Updated README and CI guidance so normal Go test commands exercise
  Testcontainers automatically.

## Reflection

Package-scoped lazy containers balance isolation and startup cost: packages do
not share database processes, while tests inside a package avoid repeatedly
starting MongoDB. Distinguishing an absent Docker daemon from a broken healthy
provider keeps local no-Docker runs usable without hiding real infrastructure
regressions.

## Decisions

- Pin tests and local setup guidance to MongoDB 8.
- Keep explicit external MongoDB support for constrained or pre-provisioned
  environments.
- Treat lifecycle failures as test failures whenever Docker is available.

## Verification

- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `golangci-lint run`
- Passed: `go mod tidy` with no residual module diff.
- Three real MongoDB containers ran, one for each integration-test package, and
  all three were cleaned up successfully.

## Next Steps

- Monitor CI duration and container startup reliability after rollout.
