#!/bin/bash
[[ -z $(git status -s) ]] || (echo "git is not clean" && exit 1);
VERSION=`git describe --tags`
BV_VERSION=${VERSION#v}
docker build --build-arg VERSION=$VERSION -t harbor.bianfeng.com/library/bumpversion:$BV_VERSION .
docker tag harbor.bianfeng.com/library/bumpversion:$BV_VERSION harbor.bianfeng.com/library/bumpversion:latest
docker push harbor.bianfeng.com/library/bumpversion:$BV_VERSION
docker push harbor.bianfeng.com/library/bumpversion:latest