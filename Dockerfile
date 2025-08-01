FROM golang:1.24-alpine AS build

WORKDIR /app
COPY . .

RUN go mod tidy
RUN go build -o volare cmd/volare/main.go

FROM alpine:3.21.3 AS prod

COPY --from=build /app/volare /usr/local/bin/volare

ENTRYPOINT [ "volare" ]