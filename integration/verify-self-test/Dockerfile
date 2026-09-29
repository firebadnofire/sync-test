FROM golang:1.22-alpine AS build
WORKDIR /src
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/verify-self-test ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/verify-self-test /usr/local/bin/verify-self-test
ENTRYPOINT ["/usr/local/bin/verify-self-test"]
