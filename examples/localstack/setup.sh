#!/bin/bash

set -eux

print_red() {
    echo -e "\033[0;31m$1\033[0m"
}

print_green() {
    echo -e "\033[0;32m$1\033[0m"
}

if command -v brew &> /dev/null
then
    echo "Brew is installed, proceeding with awscli and terraform installation."
    brew install awscli awscli-local terraform terraform-local localstack/tap/localstack-cli
else
    print_red "Brew is not installed please install awscli and terraform manually"
    exit 1
fi

if command -v npm &> /dev/null
then
    echo "Installing aws-cdk and aws-cdk-local."
    npm install -g aws-cdk aws-cdk-local
else
    print_red "npm is not installed, please install it and then return"
    exit 1
fi

if command -v go &> /dev/null
then
    echo "Installing telophasecli..."
    go install github.com/santiago-labs/telophasecli@latest
else
    print_red "go is not installed. Please install the language then return."
    exit 1
fi

localstack start -d

# Pre-create tf-test-state table to avoid concurrency bug in localstack.
aws dynamodb create-table --table-name tf-test-state \
--attribute-definitions AttributeName=id,AttributeType=S \
--key-schema AttributeName=id,KeyType=HASH \
--provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
--endpoint-url http://localhost:4566

awslocal organizations create-organization --feature-set ALL

print_green "Setup complete! You can now run telophasecli :)"
