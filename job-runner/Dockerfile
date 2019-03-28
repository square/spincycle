FROM golang:1.11.5-alpine3.9

# Copy source code and copy dev jobs over open-source stub jobs
COPY .          /go/src/github.com/square/spincycle/
COPY dev/jobs/  /go/src/github.com/square/spincycle/jobs/

# Build job-runner bin
WORKDIR /app/spin-jr/bin
RUN go build -o job-runner github.com/square/spincycle/job-runner/bin/

# Change to root of workdir and copy static files from source code
WORKDIR /app/spin-jr

EXPOSE 32307/tcp

CMD ["bin/job-runner"]
