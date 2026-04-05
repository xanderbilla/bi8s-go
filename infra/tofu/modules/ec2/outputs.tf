output "instance_id" {
  description = "EC2 instance ID"
  value       = aws_instance.this.id
}

output "instance_private_ip" {
  description = "EC2 instance private IP"
  value       = aws_instance.this.private_ip
}

output "instance_public_ip" {
  description = "EC2 instance public IP (if EIP created)"
  value       = var.create_eip ? aws_eip.this[0].public_ip : aws_instance.this.public_ip
}

output "eip_id" {
  description = "Elastic IP ID"
  value       = var.create_eip ? aws_eip.this[0].id : null
}
