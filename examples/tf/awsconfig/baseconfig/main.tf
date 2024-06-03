data "aws_region" "this" {}

data "aws_caller_identity" "this" {}

data "aws_partition" "current" {}

locals {
  is_global_recorder_region = var.global_resource_collector_region == data.aws_region.this.name
  partition                 = data.aws_partition.current.partition
}

resource "aws_config_configuration_recorder" "recorder" {
  name     = "telophase-configuration-recorder"
  role_arn = var.iam_role

  recording_group {
    all_supported                 = true
    include_global_resource_types = local.is_global_recorder_region
  }
}


resource "aws_config_delivery_channel" "this" {
  name           = "telophase-config-delivery-channel"
  s3_bucket_name = var.bucket_name

  depends_on = [aws_config_configuration_recorder.recorder]
}
