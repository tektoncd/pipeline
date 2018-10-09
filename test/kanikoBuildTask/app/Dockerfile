FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/knative/build-pipeline

# Build the outyet command inside the container.
# (You may fetch or manage dependencies here,
# either manually or with a tool like "godep".)
RUN go install github.com/knative/build-pipeline/test/kanikoBuildTask/app

# Run the outyet command by default when the container starts.
ENTRYPOINT /go/bin/app

# Document that the service listens on port 8080.
EXPOSE 8080