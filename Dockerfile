FROM golang:1.21

WORKDIR /telophasecli

COPY ./ /telophasecli

RUN go mod download

# Build the Go app
RUN go build -o telophasecli
RUN alias telophase="./telophasecli"

RUN apt-get update
# We need npm to install the CDK and terraform
RUN apt-get install -y npm
RUN npm install -g aws-cdk

# Install Terraform https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli
RUN apt-get install -y gnupg software-properties-common
RUN wget -O- https://apt.releases.hashicorp.com/gpg | \
    gpg --dearmor | \
    tee /usr/share/keyrings/hashicorp-archive-keyring.gpg > /dev/null

RUN gpg --no-default-keyring \
    --keyring /usr/share/keyrings/hashicorp-archive-keyring.gpg \
    --fingerprint

RUN echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] \
    https://apt.releases.hashicorp.com $(lsb_release -cs) main" | \
    tee /etc/apt/sources.list.d/hashicorp.list

RUN apt-get update
RUN apt-get install terraform
