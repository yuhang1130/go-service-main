# syntax=docker/dockerfile:1
FROM golang:1.26.7-alpine AS build
ARG SERVICE=api
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN case "$SERVICE" in api|consumer|job) ;; *) echo "invalid SERVICE=$SERVICE"; exit 1;; esac && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.Version=$VERSION -X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.Commit=$COMMIT -X github.com/yuhang1130/go-service-main/internal/foundation/buildinfo.BuildTime=$BUILD_TIME" \
      -o /out/service ./cmd/$SERVICE

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
COPY --from=build /src/configs /configs
ENV mq.consoleAppender.enabled=true
USER nonroot:nonroot
ENTRYPOINT ["/service"]
