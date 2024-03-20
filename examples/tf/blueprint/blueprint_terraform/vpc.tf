provider "aws" {
  region = "us-east-1"
}

resource "aws_vpc" "example_vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "ExampleVPC"
  }
}

resource "aws_internet_gateway" "example_igw" {
  vpc_id = aws_vpc.example_vpc.id

  tags = {
    Name = "ExampleIGW"
  }
}

resource "aws_subnet" "example_public_subnet" {
  count                   = 3
  vpc_id                  = aws_vpc.example_vpc.id
  cidr_block              = cidrsubnet(aws_vpc.example_vpc.cidr_block, 3, count.index)
  map_public_ip_on_launch = true
  availability_zone       = element(split(",", data.aws_availability_zones.available.names), count.index)

  tags = {
    Name = "PublicSubnet-${count.index}"
  }
}

resource "aws_subnet" "example_private_subnet" {
  count             = 3
  vpc_id            = aws_vpc.example_vpc.id
  cidr_block        = cidrsubnet(aws_vpc.example_vpc.cidr_block, 3, count.index + 3)
  availability_zone = element(split(",", data.aws_availability_zones.available.names), count.index)

  tags = {
    Name = "PrivateSubnet-${count.index}"
  }
}

resource "aws_route_table" "example_public_rt" {
  vpc_id = aws_vpc.example_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.example_igw.id
  }

  tags = {
    Name = "PublicRouteTable"
  }
}

resource "aws_route_table_association" "example_public_rta" {
  count          = length(aws_subnet.example_public_subnet.*.id)
  subnet_id      = aws_subnet.example_public_subnet.*.id[count.index]
  route_table_id = aws_route_table.example_public_rt.id
}

# Fetching availability zones
data "aws_availability_zones" "available" {}

