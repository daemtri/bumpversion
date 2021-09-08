FROM harbor.bianfeng.com/library/golang:1.17-alpine AS Builder
ARG TAGS="timetzdata"
ARG VERSION
WORKDIR /src
ENV CGO_ENABLED=0
ENV GOPRIVATE="hub.imeete.com,git.imeete.com,git.bianfeng.com"
ENV GOPROXY="https://goproxy.io"

# 缓存
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -tags "${TAGS}" -ldflags "-X main.Version=${VERSION}" -o /src/dist/bumpversion

FROM harbor.bianfeng.com/library/alpine:3.13
RUN apk add --no-cache bind-tools \
    && apk add --no-cache openssh-client \
    && ssh-keyscan git.imeete.com > /etc/ssh/ssh_known_hosts \
    && dig -t a +short git.imeete.com | grep ^[0-9] | xargs -r -n1 ssh-keyscan >> /etc/ssh/ssh_known_hosts \
    && apk del bind-tools
WORKDIR /data
ENV TZ=Asia/Shanghai
COPY --from=Builder --chown=0:0 /src/dist /usr/local/bin

ENTRYPOINT [ "bumpversion" ]