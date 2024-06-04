terraform {
  backend "s3" {
    bucket = "terraform-state-${telophase.account_id}"
    key    = "configaggregator/terraform.tfstate"
    region = "us-west-2"
  }
}


provider "aws" {
  region = "us-east-1"
}
