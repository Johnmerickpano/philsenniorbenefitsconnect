# SKELETON (M0). Becomes buildable in M2 when go.mod and cmd/api exist.
ARG GO_VERSION=1.27.3

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.20
RUN adduser -D -u 10001 appuser && apk add --no-cache ca-certificates
USER appuser
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
