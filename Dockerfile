FROM golang:latest AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY *.go ./test1
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/app

FROM alpine:latest AS release
WORKDIR /

COPY --from=build /app/app /app

ENTRYPOINT ["/app"]
