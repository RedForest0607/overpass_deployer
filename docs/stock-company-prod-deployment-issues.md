# Stock Company Prod Deployment Issues

This document records issues found during the 2026-05-27 stock-company prod deployment and Elasticsearch recovery work.

## 1. Docker daemon permission denied

### Symptom

`ec2-user` failed to run Docker Compose:

```text
permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock
```

### Cause

`ec2-user` was not allowed to access `/var/run/docker.sock`.

### Mitigation

Run with `sudo` for immediate recovery:

```bash
sudo docker compose up -d
```

For persistent non-sudo Docker access:

```bash
sudo usermod -aG docker ec2-user
```

The SSH session must be restarted after changing group membership.

## 2. Docker Compose `version` warning

### Symptom

Docker Compose printed:

```text
the attribute `version` is obsolete
```

### Cause

Recent Docker Compose ignores the top-level `version` attribute.

### Mitigation

Remove `version: "..."` from Compose files when editing them next. This warning does not block deployment.

## 3. Target-side bastion known_hosts warning

### Symptom

Search deployment completed, but target-side bastion host-key registration was skipped:

```text
Skipping bastion known_hosts registration for 10.10.0.187:22
```

### Cause

The target host could not complete `ssh-keyscan` against the bastion address. Likely causes are target-to-bastion security group, route, or network policy restrictions.

### Impact

The deployment itself completed. Bastion alias sync also completed. Only target-side known_hosts registration remained incomplete.

### Follow-up

Check whether search targets are expected to initiate SSH traffic to the bastion. If yes, allow target-to-bastion TCP/22 or adjust deployer behavior so this warning is explicitly optional for prod.

## 4. `/app/elasticsearch` archive upload permission denied

### Symptom

The deployer failed while uploading the Elasticsearch archive:

```text
creating remote file /app/elasticsearch/elasticsearch.tar: permission denied
```

### Cause

`/app/elasticsearch` is owned by the service account `elasticsearch:elasticsearch`, while the deployer connects as `ec2-user`.

### Mitigation Used

Staged the archive under `/tmp` on each target, then extracted with sudo:

```bash
scp elasticsearch.tar ec2-user@<target>:/tmp/elasticsearch.tar
ssh ec2-user@<target> 'sudo tar -xf /tmp/elasticsearch.tar -C /app && sudo chown -R elasticsearch:elasticsearch /app/elasticsearch'
```

### Follow-up

Improve `extra_files[].extract` to support a privileged extraction flow:

- upload to a writable staging path such as `/tmp`
- verify checksum
- extract with sudo
- apply final ownership
- optionally keep or remove the staged archive by policy

## 5. Elasticsearch user missing

### Symptom

Elasticsearch start script printed:

```text
sudo: unknown user elasticsearch
```

### Cause

The target host did not have the expected `elasticsearch` service account.

### Mitigation

Create the user and group with the same UID/GID as the existing server:

```bash
sudo groupadd -g 992 elasticsearch
sudo useradd -u 992 -g 992 -M -d /home/elasticsearch -s /sbin/nologin elasticsearch
sudo chown -R elasticsearch:elasticsearch /app/elasticsearch
```

### Follow-up

Add declarative service-user creation to the bootstrap layer or document it as a prerequisite for search hosts.

## 6. Broken bundled JDK under Elasticsearch

### Symptom

Elasticsearch failed because the bundled JDK was incomplete:

```text
/app/elasticsearch/es-master/jdk/bin/java: No such file or directory
```

### Cause

The deployed archive did not contain the full JDK content, or a partial cleanup left only `jdk/release`.

### Mitigation

Rebuilt the Elasticsearch archive from the existing server source archive that contained:

```text
elasticsearch/es-master/jdk/bin/java
elasticsearch/es-data/jdk/bin/java
```

Validation command:

```bash
sudo -u elasticsearch /app/elasticsearch/es-master/jdk/bin/java -version
```

## 7. macOS metadata files broke Java module loading

### Symptom

Elasticsearch failed during boot:

```text
java.lang.module.FindException: Error reading /app/elasticsearch/es-master/lib/._joni-2.1.29.jar
Caused by: java.util.zip.ZipException: zip END header not found
```

### Cause

macOS AppleDouble metadata files such as `._*.jar` were included in the archive and Java tried to load them as JARs.

### Mitigation

Remove macOS metadata from the unpacked tree before packaging:

```bash
find elasticsearch -type f \( -name '._*' -o -name '.DS_Store' \) -delete
find elasticsearch -type d -name '__MACOSX' -prune -exec rm -rf {} +
```

Package with metadata disabled on macOS:

```bash
COPYFILE_DISABLE=1 tar --no-xattrs -czf elasticsearch.tar.gz elasticsearch
```

## 8. Runtime data and logs included in Elasticsearch archive

### Symptom

The source archive contained runtime state under Elasticsearch and Kibana paths:

```text
data/
logs/
tmp/
```

### Risk

Including runtime state can cause stale cluster metadata, unexpected indices, lock files, old logs, or copied node state to be deployed to a fresh host.

### Mitigation

Rebuilt `ops/stock_company/common/extra_files/elasticsearch.tar.gz` and `ops/stock_company/prod/extra_files/elasticsearch.tar.gz` from `infra/istock-mall/elasticsearch.tar`, removing runtime contents under:

```text
*/data/*
*/logs/*
*/tmp/*
*/pid/*
*/optimize/*
```

The directories themselves were preserved.

### Validation

Verified that critical runtime/application files still exist:

```text
elasticsearch/es-master/jdk/bin/java
elasticsearch/es-data/jdk/bin/java
elasticsearch/es-server.sh
elasticsearch/kibana-server.sh
elasticsearch/kibana-8.12.0/bin/kibana
```

## 9. Elasticsearch `vm.max_map_count` too low

### Symptom

Elasticsearch refused to start:

```text
vm.max_map_count [65530] is too low, increase to at least [262144]
```

### Mitigation

Apply immediately:

```bash
sudo sysctl -w vm.max_map_count=262144
```

Persist across reboot:

```bash
echo 'vm.max_map_count=262144' | sudo tee /etc/sysctl.d/99-elasticsearch.conf
sudo sysctl --system
```

### Follow-up

Add `vm.max_map_count` to bootstrap for search hosts.

## 10. Kibana processes running as root

### Symptom

Kibana processes were running as `root` and did not stop with a non-sudo `kill`.

```text
root ... /app/elasticsearch/kibana-8.12.0/bin/../node/bin/node ...
```

### Mitigation

Stop with sudo:

```bash
sudo pkill -TERM -f '/app/elasticsearch/kibana-8.12.0'
sleep 3
sudo pkill -KILL -f '/app/elasticsearch/kibana-8.12.0'
```

### Follow-up

Run Kibana under a dedicated service user and replace ad-hoc script starts with systemd units.

## 11. Cluster yellow due to one data node

### Symptom

Cluster health was yellow:

```json
"number_of_nodes" : 2,
"number_of_data_nodes" : 1,
"unassigned_shards" : 19
```

Allocation explain showed replica shards could not be placed on the same data node:

```text
same_shard: a copy of this shard is already allocated to this node
```

### Cause

Only one data node was visible to the cluster. Replica shards cannot be assigned to the same node that already holds the primary shard.

### Mitigation Options

Preferred: bring the second data node into the same cluster.

Temporary for single-data-node operation:

```bash
curl -X PUT localhost:9200/_all/_settings \
  -H 'Content-Type: application/json' \
  -d '{"index":{"number_of_replicas":0}}'
```

### Follow-up

Confirm intended topology:

```text
10.10.50.106: es-master + es-data
10.10.51.10 : es-master + es-data
```

Then verify:

```bash
curl -s 'localhost:9200/_cat/nodes?v&h=ip,name,node.role,master,port'
curl -s localhost:9200/_cat/master?v
curl -s localhost:9200/_cluster/health?pretty
```

## 12. Cluster discovery may only see local nodes

### Symptom

Expected topology is 4 Elasticsearch nodes across two hosts, but the observed cluster had only 2 nodes and 1 data node.

### Likely Causes

- `discovery.seed_hosts` only points to the local server
- transport ports `9300` and `9301` are blocked between search hosts
- `network.host` or publish host is wrong
- duplicated `node.name` across hosts
- stale copied `data/` directory caused node identity or cluster metadata issues

### Checks

```bash
grep -E '^(cluster.name|node.name|node.roles|network.host|http.port|transport.port|discovery.seed_hosts|cluster.initial_master_nodes):' \
  /app/elasticsearch/es-master/config/elasticsearch.yml \
  /app/elasticsearch/es-data/config/elasticsearch.yml
```

```bash
nc -vz 10.10.51.10 9300
nc -vz 10.10.51.10 9301
nc -vz 10.10.50.106 9300
nc -vz 10.10.50.106 9301
```

## 13. Start scripts can print success after failure

### Symptom

Scripts printed messages such as:

```text
es-master started!!
```

even after Java/Elasticsearch startup errors.

### Cause

The start scripts do not appear to propagate child process failure reliably.

### Mitigation

Always verify after start:

```bash
ps -ef | grep elasticsearch | grep -v grep
curl -s localhost:9200/_cluster/health?pretty
tail -100 /app/elasticsearch/es-master/logs/es-cluster.log
```

### Follow-up

Refactor start scripts or replace them with systemd units that track process status and exit codes.
