# AWS Test Terraform

This directory creates EC2 resources for overpass-deployer VM-mode smoke tests on top of the existing `default-vpc` network in `ap-northeast-2`.

The network resources are represented as Terraform resources with import blocks in `imports.tf`. Terraform should import and track them, not create replacement VPC networking.

## Imported Network

- VPC: `vpc-0f509fefafc84dba6` (`10.0.0.0/16`, tag `Name=default-vpc`)
- Internet Gateway: `igw-0c27d2c7ce4b62d17`
- Public subnets:
  - `ap-northeast-2a`: `subnet-06f67ab184687f3e6` (`10.0.0.0/20`)
  - `ap-northeast-2b`: `subnet-03af2ec09baf0015d` (`10.0.16.0/20`)
- Private subnets:
  - `ap-northeast-2a`: `subnet-068d4f94971434c43` (`10.0.128.0/20`)
  - `ap-northeast-2b`: `subnet-04d6bf5d444b3a6cf` (`10.0.144.0/20`)
- Route tables:
  - Public: `rtb-013e0f8fa8ad6bccb`
  - Private `ap-northeast-2a`: `rtb-0d8974104d35f1c74`
  - Private `ap-northeast-2b`: `rtb-0624c1824990cf48b`
  - Main: `rtb-038039bb5c9b1fdef`
- Default security group: `sg-099203b95b3178ec0`

The private route tables currently include an S3 VPC endpoint route to `vpce-01bd75ecfc6ee7798` through prefix list `pl-78a54011`. They do not have a NAT route.

Imported network resources use `prevent_destroy` and `ignore_changes = all` so this test stack can reference them without trying to mutate or destroy the shared base network.

## Created Test Resources

- 1 bastion EC2 instance in public subnet `ap-northeast-2a`
- 1 private target EC2 instance per imported private subnet
- 1 EC2 key pair from your local public key
- 2 security groups for bastion and target SSH access

## Defaults

- Region: `ap-northeast-2`
- AMI: latest Amazon Linux 2023 for `x86_64`
- Instance type: `t3.nano`
- Root volume: encrypted 8 GiB gp3
- Target setup: Amazon Corretto 17 installation disabled because imported private subnets do not have a NAT route

Use `t4g.nano` only with an ARM AMI:

```bash
terraform apply \
  -var='key_name=overpass-deployer-test' \
  -var='instance_type=t4g.nano' \
  -var='ami_architecture=arm64'
```

## Usage

Create `terraform.tfvars` locally:

```hcl
key_name          = "overpass-deployer-test"
public_key_path   = "~/.ssh/id_rsa.pub"
allowed_ssh_cidrs = ["203.0.113.10/32"]
install_java      = false
```

Then run:

```bash
terraform init
terraform plan
terraform apply
```

The first `terraform apply` imports the network resources declared in `imports.tf`, then creates the EC2 test resources. Review the plan carefully before approving it, especially any proposed changes to imported VPC, subnet, route table, or internet gateway resources.

After apply, use the outputs:

```bash
terraform output bastion_ssh_command
terraform output target_private_ips
terraform output deploy_yml_hint
```

Run overpass-deployer from the bastion host, because target hosts are meant to be reached by their private IPs. This Terraform does not upload your private key to the bastion; place a test-only key there yourself if needed and keep `ssh.key_path` pointed at that file.

In `deploy.yml`, set:

```yaml
ssh:
  user: ec2-user
  key_path: ~/.ssh/id_rsa
  host_key_checking: accept-new
  port: 22

bastion:
  host: <bastion_public_ip>
  user: ec2-user
  alias_user: ec2-user

servers:
  - host: <target_private_ip>
    name: overpass-target-01
    ssh_port: 22
    bastion_host: <target_private_ip>
    bastion_ssh_port: 22
```

Destroy only the test resources when testing is done. Because this directory imports existing network resources, review any destroy plan carefully before approving it.
