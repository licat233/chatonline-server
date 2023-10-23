#!/bin/bash

#进入monitor mode
set -m

current_path=$(
    cd $(dirname $0)
    pwd
)

cd $current_path

git add --all
git commit -m "更新原origin的代码" --no-verify
git push origin1 main

# git checkout --orphan latest_branch
# git add -A
# git commit -am newest
# git branch -D main
# git branch -m main

# git fetch --all
# git reset --hard origin/main
# git pull origin1 main
