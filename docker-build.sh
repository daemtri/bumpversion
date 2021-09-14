export BV_VERSION=`git describe --abbrev=0 --tags`
docker build --build-arg VERSION=$BV_VERSION -t harbor.bianfeng.com/library/bumpversion:$BV_VERSION .