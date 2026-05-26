output "account" {
  description = "AWS account and region verified by this inventory."
  value = {
    expected_account_id = var.expected_account_id
    actual_account_id   = data.aws_caller_identity.current.account_id
    region              = var.aws_region
    profile             = var.aws_profile
  }
}

output "vpcs" {
  description = "VPCs that are relevant to stock_company deploy resources."
  value = {
    for key, vpc in local.vpcs : key => merge(vpc, {
      arn = data.aws_vpc.selected[key].arn
    })
  }
}

output "subnets" {
  description = "Subnet inventory used by the deploy targets and surrounding network."
  value = {
    for key, subnet in local.subnets : key => merge(subnet, {
      actual_available_ip_count = data.aws_subnet.selected[key].available_ip_address_count
      actual_map_public_ip      = data.aws_subnet.selected[key].map_public_ip_on_launch
    })
  }
}

output "instances" {
  description = "EC2 instances matched to deploy.yml server roles."
  value = {
    for key, instance in local.instances : key => merge(instance, {
      actual_state         = data.aws_instance.selected[key].instance_state
      actual_instance_type = data.aws_instance.selected[key].instance_type
      actual_private_ip    = data.aws_instance.selected[key].private_ip
      actual_public_ip     = data.aws_instance.selected[key].public_ip
      actual_key_name      = data.aws_instance.selected[key].key_name
    })
  }
}

output "security_groups" {
  description = "Security groups referenced by the deploy targets."
  value = {
    for key, group_id in local.security_groups : key => {
      id          = group_id
      name        = data.aws_security_group.selected[key].name
      description = data.aws_security_group.selected[key].description
      vpc_id      = data.aws_security_group.selected[key].vpc_id
    }
  }
}

output "load_balancers" {
  description = "Existing ALB resources used by stock_company deploy environments."
  value = {
    for key, name in local.load_balancers : key => {
      name     = name
      arn      = data.aws_lb.selected[key].arn
      dns_name = data.aws_lb.selected[key].dns_name
      type     = data.aws_lb.selected[key].load_balancer_type
      scheme   = data.aws_lb.selected[key].internal ? "internal" : "internet-facing"
      vpc_id   = data.aws_lb.selected[key].vpc_id
    }
  }
}

output "databases" {
  description = "Existing RDS instances discovered in the account."
  value = {
    for key, identifier in local.databases : key => {
      identifier = identifier
      address    = data.aws_db_instance.selected[key].address
      engine     = data.aws_db_instance.selected[key].engine
    }
  }
}

output "deploy_placeholder_values" {
  description = "Values for ops/stock_company/*/deploy.yml placeholder replacement."
  value       = local.deploy_placeholder_values
}

output "resource_ids" {
  description = "Route table and NAT gateway IDs that were observed and should be referenced, not recreated, unless ownership changes."
  value = {
    route_tables = local.route_tables
    nat_gateways = local.nat_gateways
  }
}
