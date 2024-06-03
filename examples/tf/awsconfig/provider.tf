terraform {
  backend "s3" {
    bucket = "terraform-state-${telophase.account_id}"
    key    = "terraform.tfstate"
    region = "us-east-1"
  }
}
