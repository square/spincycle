FROM golang:1.11.5-alpine3.9

# Copy all source code
COPY . /go/src/github.com/square/spincycle/

# Build job-runner bin
WORKDIR /app/spin-jr/bin
RUN go build -o job-runner github.com/square/spincycle/job-runner/bin/

# Change to root of workdir and copy static files from source code
WORKDIR /app/spin-jr
COPY job-runner/config/ ./config/

EXPOSE 32307/tcp

CMD ["bin/job-runner", "config/development.yaml"]
