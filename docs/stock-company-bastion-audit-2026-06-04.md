# stock-company bastion audit - 2026-06-04

## Summary

PRD/DEV bastion and target servers were audited with read-only SSH commands. The audit compared the bastion deployment assets and the current local deployment policy, then classified target state as `OK`, `MISSING`, `CHECKSUM_DIFF`, `BOOTSTRAP_MISSING`, or `SKIP_EXCLUDED`.

Important baseline mismatch:

- PRD bastion `/home/ec2-user/overpass-prod-deploy/ops/stock_company/prod/deploy.yml` checksum is `626eab7c8b1579db439b0db81529f53f322e1f1d6fd9f0d75b5aeec93a2f864e`, while local `ops/stock_company/prod/deploy.yml` checksum is `3baa2a43e3deecbc8c5cea89c8ab13305f515ed7a96de7960520303ebfcbc0f9`.
- DEV bastion `/home/ec2-user/overpass-istockmall-dev-build/ops/stock_company/dev/deploy.yml` checksum is `c2173647f510f62432ad61b245ae824ab9310cfff2b9497b11d77f2e9a745748`, while local `ops/stock_company/dev/deploy.yml` checksum is `f049c9ab929177a73bcbc5f228d54cad2d0d1ee85c56264d43d8ec4ae84b8513`.
- Both bastion deploy files are stale: they still include Kafka deploy entries and use older `conf/` config paths without per-service env files. Use the local policy as the intended next deployment baseline, after syncing it to bastion.

## Safe deployment execution - 2026-06-04

DEV was applied first, then PRD. In both environments the latest local deploy assets were synced to bastion, the deploy file was checked for Kafka/Kafka Connect, ScyllaDB, and Redis Sentinel references, dry-run was executed, the limited server subset was deployed, full bastion aliases were restored, and read-only audit was run.

| Environment | Step | Result |
| --- | --- | --- |
| DEV | Asset sync | `ops/stock_company/dev`, `ops/stock_company/common`, `TEST/stock_company/aws-rotating-smoke/overpass_setup`, and the Linux `deploy` binary were synced to `54.116.159.65:/home/ec2-user/overpass-istockmall-dev-build`. Remote `deploy.yml` checksum matched local `f049c9ab929177a73bcbc5f228d54cad2d0d1ee85c56264d43d8ec4ae84b8513`. |
| DEV | Exclusion check | Remote DEV `deploy.yml` had no `kafka`, `pionnet-kafka-connect`, `scylla`, or `sentinel` deploy references. |
| DEV | Dry-run | `./deploy vm --dry-run --config ops/stock_company/dev/deploy.yml --server-tag was,app,build` targeted only `devwas`, `devapp`, and `devbuild`. |
| DEV | Subset deploy | `devwas`, `devapp`, and `devbuild` completed. PostgreSQL service start was not run. |
| DEV | Full alias recovery | `./deploy vm --bastion-only --config ops/stock_company/dev/deploy.yml` completed. Aliases confirmed for `devapp`, `devbo-app`, `devbuild`, `devwas`, `hamonica`, `hamonica-adm`, and `postgresql`. |
| DEV | Read-only audit | `devwas`, `devapp`, and `devbuild` returned `OK_ALL_TARGETED_FILES`. |
| PRD | Asset sync | `ops/stock_company/prod`, `ops/stock_company/common`, `TEST/stock_company/aws-rotating-smoke/overpass_setup`, the Linux `deploy` binary, and the PRD key file needed by the remote config were synced to `15.164.12.24:/home/ec2-user/overpass-prod-deploy`. Remote `deploy.yml` checksum matched local `3baa2a43e3deecbc8c5cea89c8ab13305f515ed7a96de7960520303ebfcbc0f9`. |
| PRD | Exclusion check | Remote PRD `deploy.yml` had no `kafka`, `pionnet-kafka-connect`, `scylla`, or `sentinel` deploy references. |
| PRD | Dry-run | `./deploy vm --dry-run --config ops/stock_company/prod/deploy.yml --server-tag app,bo` targeted only `app1`, `app2`, `bo1`, and `bo2`. `search1/search2` Elasticsearch archive was not included. |
| PRD | Subset deploy | `app1`, `app2`, `bo1`, and `bo2` file/bootstrap deployment completed. The command returned non-zero only after target completion because the final filtered bastion alias sync attempted to reconnect from the bastion to its own public IP and timed out. |
| PRD | Full alias recovery | Retried from the local machine with `/private/tmp/overpass-deploy-darwin vm --bastion-only --config ops/stock_company/prod/deploy.yml`; it completed successfully. Aliases confirmed for `cache1`, `cache2`, `app1`, `app2`, `bo1`, `bo2`, `bo-app1`, `bo-app2`, `search1`, and `search2`. |
| PRD | Read-only audit | `app1`, `app2`, `bo1`, and `bo2` returned `OK_ALL_TARGETED_FILES`. `bo1` and `bo2` both reported `TIMEZONE Asia/Seoul (KST, +0900)` and `SWAP OK`. |

Notes:

- Application processes were not started. Real `config/<service>.env` values still need confirmation before Spring Boot startup.
- Remote deletion, Kafka cleanup, PostgreSQL service start, and Elasticsearch archive replacement were not performed.
- PRD target deployments emitted warnings while trying to register the bastion public host key from private targets. Those warnings did not block file/bootstrap deployment, and full bastion alias recovery succeeded from the local machine.

## PRD findings

| Server | Status | Confirmed state |
| --- | --- | --- |
| `cache1`, `cache2` | `MISSING` | `/home/ec2-user/software` does not exist. MongoDB and Hazelcast archive files declared by deploy policy are missing. Timezone, swap, and Corretto 25 are applied. |
| `app1` | `MISSING` | Hamonica agent jar is present. `batch-api1` and `overpass-payments-api` jar/config/env/Hamonica config/server.sh files are missing under latest `config/` policy paths. |
| `app2` | `MISSING` | `software.tar` and Hamonica agent jar are present. `batch-api2` and `overpass-payments-api` jar/config/env/Hamonica config/server.sh files are missing under latest `config/` policy paths. |
| `bo1`, `bo2` | `MISSING`, `BOOTSTRAP_MISSING` | Hamonica agent jar and all `bo-pcweb`, `po-pcweb`, `oa`, `po-mobile` app files are missing. Timezone is still UTC/n/a and swap is absent. Corretto 25 is present. |
| `bo-app1`, `bo-app2` | `OK` | nginx is active and enabled. Java is absent as intended by `jdk.enabled: false`. Timezone and swap are applied. |
| `search1`, `search2` | `CHECKSUM_DIFF` | `/app/elasticsearch/elasticsearch.tar.gz` exists, but target checksum `0bb4f92cc4279c2cc987f917a824d0e756bdc98d1948ce72169c349d90d123f7` differs from latest local checksum `b216d61a82ad124baa0b9ab12177ccd528c5089610a7144178da47d22bcdd566`. Timezone and swap are applied; Java is absent as intended. |
| `app1`, `app2` Kafka assets | `SKIP_EXCLUDED` | `/home/ec2-user/software/kafka/docker-compose.yml` and `/home/ec2-user/software/kafka/images/pionnet-kafka-connect.tar` exist from the stale bastion policy. Do not redeploy them. Removal requires a separate approved cleanup task. |

PRD immediate apply candidates:

- Sync latest local PRD deploy assets to PRD bastion first, because current bastion deploy policy is stale.
- Deploy `app1`: `batch-api1`, `overpass-payments-api`, service env files, Hamonica config files, and `server.sh`.
- Deploy `app2`: `batch-api2`, `overpass-payments-api`, service env files, Hamonica config files, and `server.sh`.
- Deploy `bo1` and `bo2`: bootstrap timezone/swap plus Hamonica agent/config and all BO/PO/OA/mobile app files.
- Reapply `search1` and `search2` Elasticsearch archive only if the newer local archive is the intended production artifact.

PRD policy/value confirmation before apply:

- Per-service `config/<service>.env` files are currently local samples. Confirm real production values before starting Spring Boot.
- Confirm whether missing cache archives should be re-uploaded or whether extracted/runtime cache services are managed outside this deployer.

## DEV findings

| Server | Status | Confirmed state |
| --- | --- | --- |
| `devwas` | `MISSING` | Timezone, swap, and Corretto 25 are applied. Hamonica agent/config files and generated `server.sh` files exist. Jars, `application.yml`, `logback-spring.xml`, and `config/<service>.env` are missing for `fo-pcweb`, `fo-mobile`, `fo-api`, `bo-pcweb`, `po-pcweb`, `oa`, and `po-mobile`. |
| `devbo-app` | `OK` | nginx is active and enabled. Java is absent as intended by `jdk.enabled: false`. Timezone and swap are applied. |
| `devapp` | `MISSING` | Timezone, swap, Corretto 25, Docker, Compose v2.29.7, `software.tar`, Hamonica agent/config files, and generated `server.sh` files are present. Elasticsearch archive is missing. `batch-api1` and `batch-api2` jars, `application.yml`, `logback-spring.xml`, and `config/<service>.env` are missing. |
| `devbuild` | `CHECKSUM_DIFF` | Docker is active and Compose v2.29.7 is installed. Jenkins `Dockerfile` and `plugins.txt` match local checksum. `/home/ec2-user/build/docker-compose.yml` differs from local latest policy. |
| `hamonica`, `hamonica-adm` | `OK` | MongoDB and Hazelcast archive checksums match local policy. Timezone, swap, and Corretto 25 are applied. |
| `postgresql` | `OK` | PostgreSQL 17 packages are installed: `postgresql17`, `postgresql17-contrib`, `postgresql17-server`, `postgresql17-private-libs`. Timezone and swap are applied. PostgreSQL service is currently inactive. |
| `devapp` Kafka assets | `SKIP_EXCLUDED` | DEV bastion policy still includes Kafka, but local policy has removed it. Do not redeploy Kafka. Removal requires a separate approved cleanup task. |

DEV immediate apply candidates:

- Sync latest local DEV deploy assets to DEV bastion first, because current bastion deploy policy is stale.
- Deploy `devwas` app jars, application/logback configs, and service env files for all seven WAS apps.
- Deploy `devapp` Elasticsearch archive plus `batch-api1`/`batch-api2` jars, application/logback configs, and service env files.
- Update `devbuild` `/home/ec2-user/build/docker-compose.yml` to the latest local policy if the local compose file is the intended build-server definition.

DEV policy/value confirmation before apply:

- Fill real service env values before launching apps. Current files are `.env.sample` sources.
- Decide whether PostgreSQL service should remain inactive or be enabled/started after package installation.

## Exclusions

- Kafka/Kafka Connect: do not redeploy `pionnet-kafka-connect.tar` or `kafka-docker-compose.yml`, even though stale bastion deploy files and some target files still contain them.
- ScyllaDB: keep excluded. It is not referenced by current deploy policy.
- Redis Sentinel: keep excluded from app configuration. Old extra files can be handled only in a separate cleanup task.

## Audit commands used

- PRD bastion deploy file checksum and copy: `ssh -i ops/stock_company/keys/prd-istockmall.pem ... ec2-user@15.164.12.24 sha256sum /home/ec2-user/overpass-prod-deploy/ops/stock_company/prod/deploy.yml`
- DEV bastion deploy file checksum and copy: `ssh -i ops/stock_company/keys/dev-istockmall.pem ... ec2-user@54.116.159.65 sha256sum /home/ec2-user/overpass-istockmall-dev-build/ops/stock_company/dev/deploy.yml`
- Target checks were run from each bastion with `ssh -i ~/.ssh/{prd,dev}-istockmall.pem` and read-only `timedatectl`, `swapon --show`, `java -version`, `systemctl is-active/is-enabled`, `docker compose version`, `rpm -q`, `test -f`, and `sha256sum`.

## Error cases reviewed

- DEV private bastion access: `10.20.0.81:22` timed out from local, but public bastion `54.116.159.65` worked.
- Bastion/local deploy mismatch: bastion deploy files are stale and were not treated as the desired next policy.
- Target access and permissions: all PRD/DEV targets in the current deploy files were reachable through bastion; no `UNKNOWN_ACCESS` target remained.
