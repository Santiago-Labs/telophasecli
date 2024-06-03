variable "iam_role" {
  type        = string
  description = "IAM role ARN for AWS Config"
}

variable "global_resource_collector_region" {
  type        = string
  description = "value of the global resource collector region"
  default     = "us-east-1"
}

variable "bucket_name" {
  type        = string
  description = "name of the S3 bucket"
}
