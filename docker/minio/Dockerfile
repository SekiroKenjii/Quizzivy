FROM golang:1.27-alpine AS build

ENV CGO_ENABLED=0 GOMAXPROCS=2 GOFLAGS=-p=1 GOMEMLIMIT=512MiB
RUN apk add --no-cache git
WORKDIR /src

RUN git init minio && cd minio && \
    git remote add origin https://github.com/minio/minio.git && \
    git fetch --depth=1 origin 07c3a429bfed433e49018cb0f78a52145d4bedeb && \
    git checkout --detach FETCH_HEAD
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    cd minio && go build -trimpath -ldflags="$(go run buildscripts/gen-ldflags.go)" -o /out/minio .

RUN git init mc && cd mc && \
    git remote add origin https://github.com/minio/mc.git && \
    git fetch --depth=1 origin 7394ce0dd2a80935aded936b09fa12cbb3cb8096 && \
    git checkout --detach FETCH_HEAD
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    cd mc && go build -trimpath -ldflags="$(go run buildscripts/gen-ldflags.go)" -o /out/mc .

FROM alpine:3.22
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/minio /out/mc /usr/bin/
COPY --from=build /src/minio/LICENSE /src/minio/CREDITS /licenses/minio/
COPY --from=build /src/mc/LICENSE /src/mc/CREDITS /licenses/mc/
ENTRYPOINT ["minio"]
