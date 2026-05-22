# ISTOCK-MALL AWS Inventory Terraform

This directory records the AWS resources discovered in the `ISTOCK-MALL` profile for account `380723148945` in `ap-northeast-2`.

It is intentionally read-only Terraform:

- no `resource` blocks
- no imports required for normal use
- data sources verify that the expected VPCs, subnets, EC2 instances, security groups, ALBs, and RDS instances still exist
- outputs provide the host placeholder values for `ops/stock_company/dev/deploy.yml`, `ops/stock_company/stage/deploy.yml`, and `ops/stock_company/prod/deploy.yml`

## Resource Shape

Use these VPCs for the current deploy assets:

- dev: `vpc-067cb0b0af0f4fed4` (`dev-vpc-01`, `10.20.0.0/16`)
- stage/prod stock_company: `vpc-087035e5939e270fb` (`prd-vpc-01`, `10.10.0.0/16`)
- separate istockmall production stack: `vpc-04f0895b77627e829` (`istockmall-VPC-prd`, `10.0.0.0/16`)

The deployer should use existing bastions and private EC2 targets instead of creating new hosts:

- dev bastion: `dev-bastion-ec2` private `10.20.0.81`, public `54.116.159.65`
- prod bastion: `prd-bastion-ec2` private `10.10.0.187`, public `15.164.12.24`
- dev targets: `devwas`, `devapp`, `devbuild`, `hamonica`, `hamonica-adm`, `postgresql`
- stage targets: `stg-fo-api`, `stg-fo-mobile`, `stg-fo-pcweb`
- prod targets: `cache1`, `cache2`, `app1`, `app2`, `bo1`, `bo2`, `search1`, `search2`

## Usage

```bash
terraform init
terraform plan
terraform output deploy_placeholder_values
```

The plan should only read data and produce outputs. If Terraform proposes resource creation, update, or deletion from this directory, stop and review the configuration first.

## Error Case Review

- Wrong AWS profile/account: `check.expected_account` fails if the active account is not `380723148945`.
- Resource drift or deletion: data sources fail fast when an EC2, subnet, security group, ALB, or RDS identifier is no longer present.
- Private host reachability: all deploy targets are private IPs, so deploy commands should run from the matching bastion or through the configured bastion path.
