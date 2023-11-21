terraform {
  required_providers {
    neon = {
      source = "terraform-community-providers/neon"
    }
  }
  backend "s3" {
    bucket = "tfstate-${telophase.account_id}"
    key    = "terraform.tfstate"
    region = "us-west-2"
  }
}

provider "neon" {}
