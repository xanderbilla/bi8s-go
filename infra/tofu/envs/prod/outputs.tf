output "vpc_id" {
  description = "VPC ID"
  value       = module.vpc.vpc_id
}

output "ec2_instance_id" {
  description = "EC2 instance ID"
  value       = module.ec2.instance_id
}

output "ec2_public_ip" {
  description = "EC2 public IP"
  value       = module.ec2.instance_public_ip
}

output "ec2_private_ip" {
  description = "EC2 private IP"
  value       = module.ec2.instance_private_ip
}

output "s3_bucket_name" {
  description = "S3 bucket name"
  value       = module.s3.bucket_name
}

output "dynamodb_tables" {
  description = "DynamoDB table names"
  value = {
    movie     = module.dynamodb_movie.table_name
    person    = module.dynamodb_person.table_name
    attribute = module.dynamodb_attribute.table_name
    encoder   = module.dynamodb_encoder.table_name
  }
}

output "environment_variables" {
  description = "Environment variables for application"
  value = {
    APP_ENV                   = var.environment
    AWS_REGION                = var.aws_region
    DYNAMODB_MOVIE_TABLE      = module.dynamodb_movie.table_name
    DYNAMODB_PERSON_TABLE     = module.dynamodb_person.table_name
    DYNAMODB_ATTRIBUTE_TABLE  = module.dynamodb_attribute.table_name
    DYNAMODB_ENCODER_TABLE    = module.dynamodb_encoder.table_name
    S3_BUCKET                 = module.s3.bucket_name
  }
}
