FROM harbor.bianfeng.com/library/golang:1.17-alpine AS Builder
ARG TAGS="timetzdata"
WORKDIR /src
ENV CGO_ENABLED=0
ENV GOPRIVATE="hub.imeete.com,git.imeete.com,git.bianfeng.com"
ENV GOPROXY="https://goproxy.io"

# 缓存
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -tags "${TAGS}" -o /src/dist/bumpversion

FROM harbor.bianfeng.com/library/alpine:3.13
WORKDIR /data
ENV TZ=Asia/Shanghai
COPY --from=Builder --chown=0:0 /src/dist /usr/local/bin

ENTRYPOINT [ "bumpversion" ]