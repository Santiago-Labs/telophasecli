variable "bucket_name" {
  type        = string
  description = "name of the S3 bucket"
  default     = "telophase-awsconfig-bucket-${telophase.account_id}"
}

variable "tags" {
  default     = {}
  description = "Tags to add to resources that support it"
  type        = map(string)
}
