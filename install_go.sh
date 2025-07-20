#!/bin/bash

# Install Go in user directory
GO_VERSION="1.21.6"
GO_ARCH="linux-amd64"
GO_TARBALL="go${GO_VERSION}.${GO_ARCH}.tar.gz"

echo "Installing Go ${GO_VERSION} for ${GO_ARCH}..."

# Download Go
wget -O /tmp/${GO_TARBALL} https://go.dev/dl/${GO_TARBALL}

# Extract to home directory
tar -C $HOME -xzf /tmp/${GO_TARBALL}

# Add Go to PATH
echo 'export PATH=$HOME/go/bin:$PATH' >> ~/.bashrc
echo 'export GOPATH=$HOME/go-workspace' >> ~/.bashrc
echo 'export PATH=$GOPATH/bin:$PATH' >> ~/.bashrc

# Source the bashrc to use Go immediately
source ~/.bashrc

echo "Go installed successfully!"
echo "Please run 'source ~/.bashrc' or start a new shell session to use Go."