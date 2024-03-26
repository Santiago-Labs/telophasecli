terraform {
  backend "s3" {
    bucket = "tfstate-${telophase.account_id}"
    key    = "terraform.tfstate"
    region = "us-west-2"
  }
}
