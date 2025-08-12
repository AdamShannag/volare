FROM golang:1.24.6-alpine3.22 AS build

WORKDIR /app
COPY . .

RUN go mod tidy
RUN go build -o volare -tags=disable_grpc_module -trimpath -ldflags="-s -w" cmd/volare/main.go

FROM alpine:3.22.1 AS prod

COPY --from=build /app/volare /usr/local/bin/volare

ENTRYPOINT [ "volare" ]