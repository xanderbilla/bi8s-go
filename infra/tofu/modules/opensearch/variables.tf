variable "domain_name" {
  type = string
}

variable "engine_version" {
  type    = string
  default = "OpenSearch_2.13"
}

variable "instance_type" {
  type    = string
  default = "t3.small.search"
}

variable "instance_count" {
  type    = number
  default = 1
}

variable "zone_awareness_enabled" {
  type    = bool
  default = false
}

variable "volume_type" {
  type    = string
  default = "gp3"
}

variable "volume_size" {
  type    = number
  default = 20
}

variable "vpc_id" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_group_name" {
  type = string
}

variable "allowed_security_group_ids" {
  type = list(string)
}

variable "aws_region" {
  type = string
}

variable "account_id" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}
