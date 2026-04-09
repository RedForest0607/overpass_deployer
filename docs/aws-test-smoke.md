# AWS EC2 Smoke Test

`infra/aws-test` is for validating the VM deployer against a bastion plus private EC2 targets in `ap-northeast-2`. This smoke test verifies file delivery only: mock jar upload, config upload, rendered script upload, script executable bit, bastion alias/known_hosts sync, and unchanged-file skip behavior on a second run.

## Files Added For The Smoke Test

- `TEST/aws-smoke/dist/mock-app.jar`
- `TEST/aws-smoke/config/application-smoke.yml`
- `TEST/aws-smoke/config/logback-smoke.xml`
- `TEST/aws-smoke/deploy.aws-test.yml`

## Preconditions

- `install_java = false`
- A dedicated EC2 key pair whose private key can be copied onto the bastion host
- `allowed_ssh_cidrs` includes your current public IP and does not include `0.0.0.0/0`
- Terraform apply has completed successfully for `infra/aws-test`

## 1. Provision Or Refresh The AWS Test Stack

From the repository root, prepare `infra/aws-test/terraform.tfvars`:

```hcl
key_name          = "overpass-deployer-aws-smoke"
public_key_path   = "~/.ssh/overpass-deployer-aws-smoke.pub"
allowed_ssh_cidrs = ["203.0.113.10/32"]
install_java      = false
```

Then run:

```bash
terraform -chdir=infra/aws-test init
terraform -chdir=infra/aws-test plan
terraform -chdir=infra/aws-test apply
```

Capture these values:

```bash
terraform -chdir=infra/aws-test output -raw bastion_public_ip
terraform -chdir=infra/aws-test output -raw bastion_private_ip
terraform -chdir=infra/aws-test output -json target_private_ips
```

`bastion_public_ip` is for your workstation to SSH/SCP into the bastion. `bastion_private_ip` is what `TEST/aws-smoke/deploy.aws-test.yml` should use as `bastion.host` because the `deploy` binary runs on the bastion itself.

## 2. Stage Deployer And Smoke-Test Assets Onto The Bastion

Build the binary locally:

```bash
go build -o /tmp/overpass-deploy ./cmd/deploy
```

Copy the binary, smoke-test assets, and the test-only private key to the bastion:

```bash
scp -i ~/.ssh/overpass-deployer-aws-smoke.pem /tmp/overpass-deploy ec2-user@<bastion_public_ip>:~/overpass-aws-test/deploy
scp -i ~/.ssh/overpass-deployer-aws-smoke.pem -r TEST/aws-smoke ec2-user@<bastion_public_ip>:~/overpass-aws-test/
scp -i ~/.ssh/overpass-deployer-aws-smoke.pem ~/.ssh/overpass-deployer-aws-smoke.pem ec2-user@<bastion_public_ip>:~/.ssh/overpass-aws-test.pem
```

On the bastion:

```bash
chmod 700 ~/.ssh
chmod 600 ~/.ssh/overpass-aws-test.pem
cd ~/overpass-aws-test/aws-smoke
```

Edit `deploy.aws-test.yml` and replace:

- `REPLACE_WITH_BASTION_PRIVATE_IP`
- `REPLACE_WITH_TARGET_PRIVATE_IP_AP_NORTHEAST_2A`
- `REPLACE_WITH_TARGET_PRIVATE_IP_AP_NORTHEAST_2B`

## 3. Review Gate: Dry-Run

Run:

```bash
cd ~/overpass-aws-test/aws-smoke
../deploy vm --dry-run --config ./deploy.aws-test.yml
```

Confirm the output includes:

- directory creation under `/app/overpass/mock-app`
- `mock-app.jar` transfer plan
- `application-smoke.yml` and `logback-smoke.xml` transfer plan
- rendered `server.sh` deployment plan
- bastion alias sync and target known_hosts registration plan

## 4. Real Smoke Deployment

Run:

```bash
cd ~/overpass-aws-test/aws-smoke
../deploy vm --config ./deploy.aws-test.yml
```

Expected order per target:

1. SSH connect
2. base directory setup
3. jar transfer
4. config file transfer
5. rendered `server.sh` transfer
6. `chmod +x` for `server.sh`
7. bastion sync

## 5. Verify Remote Files From The Bastion

Use each target private IP or the synced alias names:

```bash
ssh -i ~/.ssh/overpass-aws-test.pem ec2-user@<target_private_ip>
```

Verify file existence:

```bash
test -f /app/overpass/mock-app/bin/mock-app.jar
test -f /app/overpass/mock-app/config/application.yml
test -f /app/overpass/mock-app/config/logback.xml
test -f /app/overpass/mock-app/scripts/server.sh
test -x /app/overpass/mock-app/scripts/server.sh
```

Verify SHA256 values from the bastion staging directory:

```bash
sha256sum ./dist/mock-app.jar ./config/application-smoke.yml ./config/logback-smoke.xml
ssh -i ~/.ssh/overpass-aws-test.pem ec2-user@<target_private_ip> \
  "sha256sum /app/overpass/mock-app/bin/mock-app.jar /app/overpass/mock-app/config/application.yml /app/overpass/mock-app/config/logback.xml"
```

## 6. Re-Run For Skip Verification

Run the same command again:

```bash
cd ~/overpass-aws-test/aws-smoke
../deploy vm --config ./deploy.aws-test.yml
```

Review logs for unchanged-file skip behavior on:

- `mock-app.jar`
- `application.yml`
- `logback.xml`

The second run should still leave the rendered scripts present and executable.

## 7. Cleanup

On the bastion:

```bash
rm -rf ~/overpass-aws-test
rm -f ~/.ssh/overpass-aws-test.pem
rm -rf ~/.overpass-aws-smoke
```

From your workstation:

```bash
terraform -chdir=infra/aws-test plan -destroy
terraform -chdir=infra/aws-test destroy
```

Review destroy carefully. `infra/aws-test` imports shared default VPC resources and should destroy only the test EC2 resources plus test security groups and key pair.

## Notes

- The smoke-test jar is intentionally a plain mock file, not a runnable Java archive.
- Bastion security rules in `infra/aws-test` allow target-to-bastion `ssh-keyscan` and bastion self-SSH so the current M1 bastion sync flow can complete without code changes.
