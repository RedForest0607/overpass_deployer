data "aws_caller_identity" "current" {}

data "aws_vpc" "selected" {
  for_each = local.vpcs

  id = each.value.id
}

data "aws_subnet" "selected" {
  for_each = local.subnets

  id = each.value.id
}

data "aws_instance" "selected" {
  for_each = local.instances

  instance_id = each.value.instance_id
}

data "aws_security_group" "selected" {
  for_each = local.security_groups

  id = each.value
}

data "aws_lb" "selected" {
  for_each = local.load_balancers

  name = each.value
}

data "aws_db_instance" "selected" {
  for_each = local.databases

  db_instance_identifier = each.value
}

check "expected_account" {
  assert {
    condition     = data.aws_caller_identity.current.account_id == var.expected_account_id
    error_message = "This inventory must be run against the ISTOCK-MALL account ${var.expected_account_id}."
  }
}
