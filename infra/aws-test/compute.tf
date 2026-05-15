data "aws_ami" "amazon_linux_2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-kernel-6.1-${var.ami_architecture}"]
  }

  filter {
    name   = "architecture"
    values = [var.ami_architecture]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_key_pair" "this" {
  key_name   = var.key_name
  public_key = file(pathexpand(var.public_key_path))

  tags = merge(local.common_tags, {
    Name = var.key_name
  })
}

locals {
  java_user_data = <<-USERDATA
    #!/bin/bash
    set -euxo pipefail
    dnf install -y java-17-amazon-corretto-headless
  USERDATA

  target_user_data = var.install_java ? local.java_user_data : null
}

resource "aws_instance" "bastion" {
  ami                         = data.aws_ami.amazon_linux_2023.id
  instance_type               = var.instance_type
  key_name                    = aws_key_pair.this.key_name
  subnet_id                   = aws_subnet.public[local.bastion_public_subnet_az].id
  vpc_security_group_ids      = [aws_security_group.bastion.id]
  associate_public_ip_address = true

  root_block_device {
    volume_type = "gp3"
    volume_size = var.bastion_root_volume_size
    encrypted   = true
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-bastion"
    Role = "bastion"
  })
}

resource "aws_instance" "target" {
  for_each = var.dev_targets

  ami                         = data.aws_ami.amazon_linux_2023.id
  instance_type               = var.instance_type
  key_name                    = aws_key_pair.this.key_name
  subnet_id                   = aws_subnet.private[each.value.subnet_az].id
  vpc_security_group_ids      = [aws_security_group.target.id]
  associate_public_ip_address = false
  user_data                   = local.target_user_data

  root_block_device {
    volume_type = "gp3"
    volume_size = var.target_root_volume_size
    encrypted   = true
  }

  lifecycle {
    precondition {
      condition     = !var.install_java
      error_message = "install_java must remain false with the imported private subnets because they do not have a NAT route for package mirrors."
    }
  }

  tags = merge(local.common_tags, {
    Name       = "${var.project_name}-${each.key}"
    Role       = each.key
    ServerName = each.key
    AZ         = each.value.subnet_az
    DeployTags = join(",", each.value.tags)
  })
}
