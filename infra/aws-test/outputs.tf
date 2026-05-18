output "bastion_public_ip" {
  description = "Public IP address for SSH into the bastion host."
  value       = aws_instance.bastion.public_ip
}

output "bastion_private_ip" {
  description = "Private IP address to use as bastion.host when deploy runs on the bastion itself."
  value       = aws_instance.bastion.private_ip
}

output "bastion_ssh_command" {
  description = "SSH command for the test bastion."
  value       = "ssh -i ${trimsuffix(var.public_key_path, ".pub")} ec2-user@${aws_instance.bastion.public_ip}"
}

output "target_private_ips" {
  description = "Private IP addresses by stock_company dev server name."
  value       = { for name, instance in aws_instance.target : name => instance.private_ip }
}

output "target_names" {
  description = "Suggested deploy.yml server names."
  value       = { for name in keys(aws_instance.target) : name => name }
}

output "stock_company_dev_placeholder_values" {
  description = "Values to replace stock_company dev deploy.yml host placeholders."
  value = merge(
    {
      BASTION_HOST = aws_instance.bastion.private_ip
    },
    {
      for name, instance in aws_instance.target : lookup({
        devwas = "DEVWAS_HOST"
      }, name, "${upper(name)}_HOST") => instance.private_ip
    }
  )
}

output "deploy_yml_hint" {
  description = "Minimal host block hints for deploy.yml."
  value = [
    for name, instance in aws_instance.target : {
      name         = name
      host         = instance.private_ip
      ssh_port     = 22
      bastion_host = instance.private_ip
      az           = var.dev_targets[name].subnet_az
      tags         = var.dev_targets[name].tags
    }
  ]
}
