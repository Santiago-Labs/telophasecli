#!/bin/bash

set -eux

OS=$(uname -s)

if [ "$OS" == "Linux" ]; then
    sudo apt-get update
    sudo apt-get install -y awscli
    npm install -g aws-cdk
    curl -Lo localstack-cli-3.3.0-linux-amd64-onefile.tar.gz https://github.com/localstack/localstack-cli/releases/download/v3.3.0/localstack-cli-3.3.0-linux-amd64-onefile.tar.gz
    sudo tar xvzf localstack-cli-3.3.0-linux-*-onefile.tar.gz -C /usr/local/bin
elif [ "$OS" == "Darwin" ]; then
    brew install awscli
    npm install -g aws-cdk
    brew install localstack/tap/localstack-cli
else
    echo "Unsupported operating system: $OS"
    exit 1
fi

cd ..
go build .
cd tests

localstack start -d

sleep 5

# Pre-create tf-test-state table to avoid concurrency bug in localstack.
aws dynamodb create-table --table-name tf-test-state \
--attribute-definitions AttributeName=id,AttributeType=S \
--key-schema AttributeName=id,KeyType=HASH \
--provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
--endpoint-url http://localhost:4566
