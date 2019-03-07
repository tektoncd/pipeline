FROM golang

# Copy the local package files to the container's workspace.
COPY . /go/src/github.com/tektoncd/pipeline/

RUN go install github.com/tektoncd/pipeline/test/gohelloworld

ENTRYPOINT /go/bin/gohelloworld

# Document that the service listens on port 8080.
EXPOSE 8080