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
GIT_TREE_STATE=$(test -n "$(git status --porcelain)" && echo "dirty" || echo "clean")
echo $GIT_TREE_STATE
if ! CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="-w -extldflags '-static' -X main.Commit=$GIT_COMMIT -X main.CommitDate=$GIT_COMMIT_DATE -X main.Version=$GIT_VERSION -X main.TreeState=$GIT_TREE_STATE" -installsuffix cgo -o app ./main.go; then
    echo "程序编译失败"
    exit 1
fi

echo "编译完成"

# exit 0

# docker pull alpine:latest

echo "已确保alpine镜像为最新"
echo "开始编译docker镜像: licat/chatonline-server:v3"

if ! docker buildx build --platform linux/amd64 --tag licat/chatonline-server:v3 . -f Dockerfile.lite; then
    echo "docker镜像编译失败"
    exit 1
fi

echo "docker镜像编译完成"
echo "开始推送docker镜像到远程仓库"

docker push licat/chatonline-server:v3

echo "推送完成"

function clearup_none_image() {
    # 列出所有的镜像，并获取 tag 为 "none" 的镜像的 IMAGE ID
    image_ids=$(docker images | awk '$2 == "<none>" { print $3 }')

    # 检查是否有 tag 为 "none" 的镜像
    if [[ -z "$image_ids" ]]; then
        echo "没有找到 tag 为 \"none\" 的镜像"
        exit 0
    fi

    # 删除所有 tag 为 "none" 的镜像
    for image_id in $image_ids; do
        docker rmi $image_id
    done

    echo "已成功删除所有 tag 为 \"none\" 的镜像"
}

clearup_none_image