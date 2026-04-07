resource "aws_default_security_group" "default" {
  vpc_id = aws_vpc.this.id

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_security_group" "bastion" {
  name        = "${var.project_name}-bastion-sg"
  description = "Allow SSH into the overpass-deployer test bastion."
  vpc_id      = aws_vpc.this.id

  ingress {
    description = "SSH from approved client CIDRs"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.allowed_ssh_cidrs
  }

  egress {
    description = "Allow outbound traffic for package mirrors and target SSH"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-bastion-sg"
  })
}

resource "aws_security_group" "target" {
  name        = "${var.project_name}-target-sg"
  description = "Allow SSH to test target VMs only from the bastion security group."
  vpc_id      = aws_vpc.this.id

  ingress {
    description     = "SSH from bastion"
    from_port       = 22
    to_port         = 22
    protocol        = "tcp"
    security_groups = [aws_security_group.bastion.id]
  }

  egress {
    description = "Allow outbound traffic for package mirrors"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-target-sg"
  })
}
