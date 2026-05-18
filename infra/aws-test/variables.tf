locals {
  common_tags = {
    Project     = var.project_name
    Environment = var.environment_name
    ManagedBy   = "terraform"
    Purpose     = "overpass-deployer-stock-company-dev-test"
  }
}

variable "project_name" {
  description = "Name prefix for test AWS resources."
  type        = string
  default     = "overpass-deployer-test"
}

variable "aws_region" {
  description = "AWS region where the test resources will be created."
  type        = string
  default     = "ap-northeast-2"
}

variable "key_name" {
  description = "Name for the EC2 key pair used by bastion and target instances."
  type        = string
}

variable "public_key_path" {
  description = "Path to the local SSH public key to register as an EC2 key pair."
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "allowed_ssh_cidrs" {
  description = "CIDR blocks allowed to SSH into the bastion host. Use your office/home IP, for example [\"203.0.113.10/32\"]."
  type        = list(string)
  default     = []

  validation {
    condition     = length(var.allowed_ssh_cidrs) > 0
    error_message = "allowed_ssh_cidrs must include at least one explicit client CIDR."
  }

  validation {
    condition     = !contains(var.allowed_ssh_cidrs, "0.0.0.0/0")
    error_message = "allowed_ssh_cidrs must not include 0.0.0.0/0 for this test stack."
  }
}

variable "ami_architecture" {
  description = "Amazon Linux 2023 AMI architecture. Use x86_64 with t3.nano, or arm64 with t4g.nano."
  type        = string
  default     = "x86_64"

  validation {
    condition     = contains(["x86_64", "arm64"], var.ami_architecture)
    error_message = "ami_architecture must be either x86_64 or arm64."
  }
}

variable "instance_type" {
  description = "EC2 instance type for all test instances. t3.nano is the default smallest x86_64-friendly choice."
  type        = string
  default     = "t3.nano"
}

variable "environment_name" {
  description = "Environment label for test resource tags."
  type        = string
  default     = "dev-test"
}

variable "bastion_root_volume_size" {
  description = "Root volume size in GiB for the bastion. Keep enough space for deploy binaries and copied stock-company assets."
  type        = number
  default     = 20

  validation {
    condition     = var.bastion_root_volume_size >= 8
    error_message = "bastion_root_volume_size must be at least 8 GiB."
  }
}

variable "target_root_volume_size" {
  description = "Root volume size in GiB for dev target instances. Stock-company software archives can be large, so 30 GiB is the default."
  type        = number
  default     = 30

  validation {
    condition     = var.target_root_volume_size >= 8
    error_message = "target_root_volume_size must be at least 8 GiB."
  }
}

variable "dev_targets" {
  description = "stock_company dev target server roles and private subnet placement."
  type = map(object({
    subnet_az = string
    tags      = list(string)
  }))
  default = {
    devwas = {
      subnet_az = "ap-northeast-2a"
      tags      = ["stock-company", "dev", "was", "overpass"]
    }
    devapp1 = {
      subnet_az = "ap-northeast-2a"
      tags      = ["stock-company", "dev", "app", "overpass", "batch", "search", "agents"]
    }
    devapp2 = {
      subnet_az = "ap-northeast-2b"
      tags      = ["stock-company", "dev", "app", "kafka"]
    }
    devapm1 = {
      subnet_az = "ap-northeast-2a"
      tags      = ["stock-company", "dev", "apm", "cache"]
    }
    devapm2 = {
      subnet_az = "ap-northeast-2b"
      tags      = ["stock-company", "dev", "apm", "cache"]
    }
  }

  validation {
    condition = alltrue([
      for target in values(var.dev_targets) : contains(["ap-northeast-2a", "ap-northeast-2b"], target.subnet_az)
    ])
    error_message = "Each dev_targets subnet_az must match one of the imported private subnet AZ keys."
  }
}

variable "install_java" {
  description = "Install Amazon Corretto 17 on target instances for deployer smoke tests. Disabled by default because imported private subnets do not have a NAT route."
  type        = bool
  default     = false
}
