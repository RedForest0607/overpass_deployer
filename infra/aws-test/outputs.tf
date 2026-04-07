output "bastion_public_ip" {
  description = "Public IP address for SSH into the bastion host."
  value       = aws_instance.bastion.public_ip
}

output "bastion_ssh_command" {
  description = "SSH command for the test bastion."
  value       = "ssh -i ${trimsuffix(var.public_key_path, ".pub")} ec2-user@${aws_instance.bastion.public_ip}"
}

output "target_private_ips" {
  description = "Private IP addresses to use in deploy.yml servers[].host from the bastion."
  value       = { for az, instance in aws_instance.target : az => instance.private_ip }
}

output "target_names" {
  description = "Suggested deploy.yml server names."
  value       = { for az in keys(aws_instance.target) : az => "${var.project_name}-target-${az}" }
}

output "deploy_yml_hint" {
  description = "Minimal host block hints for deploy.yml."
  value = [
    for az, instance in aws_instance.target : {
      name         = "${var.project_name}-target-${az}"
      host         = instance.private_ip
      ssh_port     = 22
      bastion_host = instance.private_ip
      az           = az
    }
  ]
}
