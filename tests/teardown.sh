#!/bin/bash

set -eux

localstack stop
rm organization.yml
rm -rf telophasedirs/