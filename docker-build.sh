export BV_VERSION=1.0.11
docker build --build-arg VERSION=$BV_VERSION -t harbor.bianfeng.com/library/bumpversion:$BV_VERSION .