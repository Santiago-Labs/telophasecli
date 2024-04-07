#!/bin/bash

set -eux

if ! nc -z localhost 4566; then
  localstack start -d
  sleep 5
fi

# Pre-create tf-test-state table to avoid concurrency bug in localstack.
aws dynamodb create-table --table-name tf-test-state \
--attribute-definitions AttributeName=id,AttributeType=S \
--key-schema AttributeName=id,KeyType=HASH \
--provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
--endpoint-url http://localhost:4566
