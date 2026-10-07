# Builds the dbclone CLI. The image carries only the static binary and the docker CLI:
# every dump and restore still runs inside your local mongo/mysql containers.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /dbclone .

FROM alpine:3.21
RUN apk add --no-cache docker-cli
COPY --from=build /dbclone /usr/local/bin/dbclone
WORKDIR /work
ENTRYPOINT ["dbclone"]
