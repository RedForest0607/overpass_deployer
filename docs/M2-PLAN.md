# M2-PLAN.md - VM Mode Completion (Idempotency)

This checklist tracks the work needed to make VM deployments repeatable and safe for operations.

## Completion Criteria

- [x] File deployment compares SHA256 and skips unchanged artifacts.
- [x] Bootstrap packages, users, sysctl, timezone, swap, Docker Compose, and services check their current state before changing it.
- [x] Package bootstrap supports RPM hosts (`dnf`/`yum`) and Ubuntu/Debian hosts (`apt-get`).
- [x] JDK package selection uses Corretto packages on RPM hosts and distribution OpenJDK packages on `apt-get` hosts.
- [x] `servers[].utils[]` deploys utility directories and files before applying its service state.
- [x] Utility declarations, service states, paths, duplicate names, and local files are validated before SSH connection.
- [x] Sequential and parallel deployments attempt every selected server, then print a server-level `done / skipped / failed` summary.
- [x] `go test ./...` and `go vet ./...` pass after the M2 changes.
- [x] Podman Go 1.26.1 container passes `go test ./internal/config ./internal/vm` and `go vet ./...` with the workspace mounted read-only.
- [x] Classify an entire server as `skipped` when every observed managed step is unchanged.
- [x] Add an integration scenario covering a second real VM deployment with no changed files (`tests/podman/run-m2-idempotency.sh`).
- [x] Verify supported Ubuntu and RPM image/JDK version combinations in the target environment: Ubuntu 24.04/OpenJDK 21 and Amazon Linux 2023/Corretto 17.

## Utility Service Declaration

Use `servers[].utils[]` for components such as Redis, Elasticsearch, Kibana, or an operational agent. A utility's order is fixed: `directories` -> `files` -> `services`.

```yaml
servers:
  - name: cache-01
    host: 10.0.1.20
    utils:
      - name: redis
        directories:
          - /app/redis
        files:
          - local_path: ./configs/redis.conf
            remote_path: /app/redis/redis.conf
        services:
          - name: redis-server
            enabled: true
            state: restarted
```

`files` follows the same checksum, permissions, archive extraction, and validation rules as `extra_files`.

## Podman Integration Verification

Run the two target scenarios from the repository root:

```bash
bash tests/podman/run-m2-idempotency.sh ubuntu
bash tests/podman/run-m2-idempotency.sh amazonlinux
```

Each scenario builds an SSH target, installs a supported JDK through the target OS package manager (Ubuntu 24.04/JDK 21 and Amazon Linux 2023/Corretto 17), deploys the same assets twice, and requires the second server summary to be `skipped`.

## Safety Notes

- Package names are OS-specific. Keep `bootstrap.packages` compatible with the target OS image.
- On Ubuntu/Debian, JDK installation uses the distribution's `openjdk-<major>-jdk[-headless]` package even when the compatibility setting is `vendor: corretto`.
- When one target fails, remaining selected targets continue so the final summary is complete. Bastion synchronization does not run after a failed target batch.
- The full Podman `go test ./...` run has two pre-existing `internal/update` test fixture failures: root bypasses a directory permission test, and Linux/ARM64 matches a fixture intended to be platform-missing. They do not affect the M2 `config`/`vm` result.
