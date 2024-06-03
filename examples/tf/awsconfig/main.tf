terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

data "aws_partition" "current" {}

locals {
  enabled_regions = [
    "ap-northeast-1",
    "ap-northeast-2",
    "ap-northeast-3",
    "ap-south-1",
    "ap-southeast-1",
    "ap-southeast-2",
    "ca-central-1",
    "eu-central-1",
    "eu-north-1",
    "eu-west-1",
    "eu-west-2",
    "eu-west-3",
    "sa-east-1",
    "us-east-1",
    "us-east-2",
    "us-west-1",
    "us-west-2",
  ]
}

# Default Provider
provider "aws" {
  region = "us-east-1"
}

module "us_east_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "us-east-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  # No need to specify provider for this because it is using the default provider
  #   providers = {
  #     aws = aws.us-east-1
  #   }
}

provider "aws" {
  alias  = "us-east-2"
  region = "us-east-2"
}

module "us_east_2" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "us-east-2") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.us-east-2
  }
}

provider "aws" {
  alias  = "us-west-1"
  region = "us-west-1"
}

module "us_west_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "us-west-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.us-west-1
  }
}

provider "aws" {
  alias  = "us-west-2"
  region = "us-west-2"
}

module "us_west_2" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "us-west-2") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.us-west-2
  }
}

provider "aws" {
  alias  = "ca-central-1"
  region = "ca-central-1"
}

module "ca_central_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ca-central-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.ca-central-1
  }
}

provider "aws" {
  alias  = "eu-central-1"
  region = "eu-central-1"
}

module "eu_central_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "eu-central-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.eu-central-1
  }
}

provider "aws" {
  alias  = "eu-west-1"
  region = "eu-west-1"
}

module "eu_west_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "eu-west-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.eu-west-1
  }
}

provider "aws" {
  alias  = "eu-west-2"
  region = "eu-west-2"
}

module "eu_west_2" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "eu-west-2") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.eu-west-2
  }
}

provider "aws" {
  alias  = "eu-west-3"
  region = "eu-west-3"
}

module "eu_west_3" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "eu-west-3") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.eu-west-3
  }
}

provider "aws" {
  alias  = "eu-north-1"
  region = "eu-north-1"
}

module "eu_north_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "eu-north-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.eu-north-1
  }
}

provider "aws" {
  alias  = "ap-northeast-1"
  region = "ap-northeast-1"
}

module "ap_northeast_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ap-northeast-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.ap-northeast-1
  }
}

provider "aws" {
  alias  = "ap-northeast-2"
  region = "ap-northeast-2"
}

module "ap_northeast_2" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ap-northeast-2") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id
  providers = {
    aws = aws.ap-northeast-2
  }
}

provider "aws" {
  alias  = "ap-northeast-3"
  region = "ap-northeast-3"
}

module "ap_northeast_3" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ap-northeast-3") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.ap-northeast-3
  }
}

provider "aws" {
  region = "ap-southeast-1"
  alias  = "ap-southeast-1"
}

module "ap_southeast_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ap-southeast-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.ap-southeast-1
  }
}

provider "aws" {
  alias  = "ap-southeast-2"
  region = "ap-southeast-2"
}

module "ap_southeast_2" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ap-southeast-2") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.ap-southeast-2
  }
}

provider "aws" {
  region = "ap-south-1"
  alias  = "ap-south-1"
}

module "ap_south_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "ap-south-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.ap-south-1
  }
}

provider "aws" {
  alias  = "sa-east-1"
  region = "sa-east-1"
}

module "sa_east_1" {
  source = "./baseconfig"

  count = contains(local.enabled_regions, "sa-east-1") ? 1 : 0

  iam_role    = aws_iam_role.iam.arn
  bucket_name = aws_s3_bucket.bucket.id

  providers = {
    aws = aws.sa-east-1
  }
}
