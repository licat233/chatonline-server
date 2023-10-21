#!/bin/bash

#进入monitor mode
set -m

current_path=$(
    cd $(dirname $0)
    pwd
)

cd $current_path

go mod tidy

echo "开始编译程序"

GIT_COMMIT=$(git rev-parse HEAD)
echo $GIT_COMMIT
GIT_COMMIT_DATE=$(git log --date=iso8601-strict -1 --pretty=%ct)
echo $GIT_COMMIT_DATE
GIT_VERSION=$(git describe --tags --abbrev=0)
echo $GIT_VERSION
GIT_TREE_STATE=$(test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
echo $GIT_TREE_STATE
CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="-w -extldflags '-static' -X main.Commit=$GIT_COMMIT -X main.CommitDate=$GIT_COMMIT_DATE -X main.Version=$GIT_VERSION -X main.TreeState=$GIT_TREE_STATE" -installsuffix cgo  -o app ./main.go

echo "编译完成"

# docker pull alpine:latest

echo "已确保alpine镜像为最新"
echo "开始编译docker镜像: licat/chatonline-server:beta"

docker buildx build --platform linux/amd64 --tag licat/chatonline-server:beta . -f Dockerfile.lite

echo "docker镜像编译完成"
echo "开始推送docker镜像到远程仓库"

docker push licat/chatonline-server:beta

echo "推送完成"