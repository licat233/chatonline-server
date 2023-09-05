#!/bin/bash

#进入monitor mode
set -m

current_path=$(
    cd $(dirname $0)
    pwd
)

cd $current_path

docker buildx build --platform linux/amd64 --tag licat/chatonline-server:latest .

docker buildx build --platform linux/arm64 --tag licat/chatonline-server:latest .

docker push licat/chatonline-server:latest
