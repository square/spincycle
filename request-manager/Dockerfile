# Build from repo root dir: docker build -f request-manager/Dockerfile . -t spin-rm
# Run one container:        docker run -p 32308:32308 spin-rm

FROM golang:1.11.5-alpine3.9

# Install mysql cli so entrypoint script can init db
RUN apk add --no-cache \
    mysql-client

# Copy source code and copy dev jobs over open-source stub jobs
COPY .          /go/src/github.com/square/spincycle/
COPY dev/jobs/  /go/src/github.com/square/spincycle/jobs/

# Build request-manager bin
WORKDIR /app/spin-rm/bin
RUN go build -o request-manager github.com/square/spincycle/request-manager/bin/

# Change to root of workdir and copy static files from source code
WORKDIR /app/spin-rm
COPY dev/docker-entrypoint.sh   ./
COPY dev/specs/                 ./specs/
COPY request-manager/resources/ ./resources/

EXPOSE 32308/tcp

ENTRYPOINT ["/app/spin-rm/docker-entrypoint.sh"]
