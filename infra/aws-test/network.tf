locals {
  imported_public_subnets = {
    "ap-northeast-2a" = {
      id         = "subnet-06f67ab184687f3e6"
      cidr_block = "10.0.0.0/20"
      name       = "default-subnet-public1-ap-northeast-2a"
    }
    "ap-northeast-2b" = {
      id         = "subnet-03af2ec09baf0015d"
      cidr_block = "10.0.16.0/20"
      name       = "default-subnet-public2-ap-northeast-2b"
    }
  }

  imported_private_subnets = {
    "ap-northeast-2a" = {
      id         = "subnet-068d4f94971434c43"
      cidr_block = "10.0.128.0/20"
      name       = "default-subnet-private1-ap-northeast-2a"
    }
    "ap-northeast-2b" = {
      id         = "subnet-04d6bf5d444b3a6cf"
      cidr_block = "10.0.144.0/20"
      name       = "default-subnet-private2-ap-northeast-2b"
    }
  }

  bastion_public_subnet_az = "ap-northeast-2a"
}

resource "aws_vpc" "this" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "default-vpc"
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "default-igw"
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_subnet" "public" {
  for_each = local.imported_public_subnets

  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value.cidr_block
  availability_zone       = each.key
  map_public_ip_on_launch = false

  tags = {
    Name = each.value.name
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_subnet" "private" {
  for_each = local.imported_private_subnets

  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value.cidr_block
  availability_zone       = each.key
  map_public_ip_on_launch = false

  tags = {
    Name = each.value.name
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = {
    Name = "default-rtb-public"
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_route_table" "private" {
  for_each = {
    "ap-northeast-2a" = {
      id   = "rtb-0d8974104d35f1c74"
      name = "default-rtb-private1-ap-northeast-2a"
    }
    "ap-northeast-2b" = {
      id   = "rtb-0624c1824990cf48b"
      name = "default-rtb-private2-ap-northeast-2b"
    }
  }

  vpc_id = aws_vpc.this.id

  route {
    destination_prefix_list_id = "pl-78a54011"
    vpc_endpoint_id            = "vpce-01bd75ecfc6ee7798"
  }

  tags = {
    Name = each.value.name
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_route_table" "main" {
  vpc_id = aws_vpc.this.id

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_route_table_association" "public" {
  for_each = local.imported_public_subnets

  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}

resource "aws_route_table_association" "private" {
  for_each = aws_subnet.private

  subnet_id      = each.value.id
  route_table_id = aws_route_table.private[each.key].id

  lifecycle {
    prevent_destroy = true
    ignore_changes  = all
  }
}
