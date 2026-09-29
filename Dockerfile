FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gh-sync ./cmd/gh-sync

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/gh-sync /usr/local/bin/gh-sync
ENTRYPOINT ["/usr/local/bin/gh-sync"]
