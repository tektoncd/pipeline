FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/knative/build-pipeline/test/gohelloworld

RUN go install github.com/knative/build-pipeline/test/gohelloworld

ENTRYPOINT /go/bin/gohelloworld

# Document that the service listens on port 8080.
EXPOSE 8080