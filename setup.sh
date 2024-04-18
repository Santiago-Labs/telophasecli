#!/bin/bash

set -eux

print_red() {
    echo -e "\033[0;31m$1\033[0m"
}

print_green() {
    echo -e "\033[0;32m$1\033[0m"
}

if command -v brew &> /dev/null; then
    echo "Brew is installed, proceeding with awscli and terraform installation."
    brew install awscli awscli-local terraform terraform-local localstack/tap/localstack-cli
elif [ "$(uname)" = "Linux" ]; then
    if [ -f /etc/debian_version ]; then
        echo "Debian-based Linux detected. Installing packages with apt."
        sudo apt update && sudo apt install -y awscli
        pip3 install awscli-local
        pip3 install terraform-local
        sudo snap install terraform --classic
        curl -Lo localstack-cli-3.3.0-linux-amd64-onefile.tar.gz https://github.com/localstack/localstack-cli/releases/download/v3.3.0/localstack-cli-3.3.0-linux-amd64-onefile.tar.gz
        sudo tar xvzf localstack-cli-3.3.0-linux-*-onefile.tar.gz -C /usr/local/bin
    elif [ -f /etc/redhat-release ]; then
        echo "Red Hat-based Linux detected. Installing packages with dnf."
        sudo dnf install -y awscli
        print_red "Please manually install Terraform following the official instructions."
    fi
else
    print_red "Brew is not installed, or you're not on a supported Linux distribution. Please install awscli and terraform manually."
    exit 1
fi

if command -v npm &> /dev/null
then
    echo "Installing aws-cdk and aws-cdk-local."
    npm install -g aws-cdk aws-cdk-local
else
    print_red "npm is not installed. Please install it and then return."
    exit 1
fi

if command -v go &> /dev/null
then
    echo "Installing telophasecli..."
    go install github.com/santiago-labs/telophasecli@latest
else
    if [ "$(uname)" = "Darwin" ]; then
        print_red "Go is not installed. On Mac, you can install Go using Homebrew with: brew install go. Then, return to run this script again."
    elif [ "$(uname)" = "Linux" ]; then
        print_red "Go is not installed. On Linux, you can generally install Go by running: sudo snap install go --classic (Debian/Ubuntu) or sudo dnf install golang (Fedora/RHEL). Then, return to run this script again."
    else
        print_red "Go is not installed. Please visit https://golang.org/doc/install for instructions on how to install Go on your system."
    fi
    exit 1
fi

print_green "Setup complete! You can now run telophasecli :)"