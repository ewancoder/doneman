FROM golang:latest AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/app

FROM alpine:latest AS release
WORKDIR /

# Copy Healthcheck executable.
ENV HEALTHCHECK_FILE=/tmp/health
COPY --from=ewancoder/healthcheck:latest /healthcheck /healthcheck
HEALTHCHECK CMD ["/healthcheck"]

COPY --from=build /app/app /app

ENTRYPOINT ["/app"]
