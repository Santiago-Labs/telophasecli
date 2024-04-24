resource "aws_dynamodb_table" "example" {
  name             = "table-${telophase.region}"

  hash_key         = "TestTableHashKey"
  billing_mode     = "PAY_PER_REQUEST"
  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"

  attribute {
    name = "TestTableHashKey"
    type = "S"
  }
}

locals {
  region = split("_",terraform.workspace)[1]
}

provider "aws" {
    # Two options can use ${telophase.region} or look at local config
    region = "${telophase.region}" 
}

terraform {
  backend "s3" {
    bucket = "tfstate-000000000000"
    key    = "workspace/terraform.tfstate"
    region = "us-east-1"
  }
}
