FROM golang:1.18 as build

WORKDIR /go-licenses

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS
ENV GO111MODULE=on

# Download dependencies first - this should be cacheable.
COPY go.mod go.sum ./
RUN go mod download

# Now add the local repo, which typically isn't cacheable.
COPY . .

# Check that all of the Go code builds
RUN go build ./...

# Run the tests
RUN go test -v ./...

# Install the binary into /go/bin
RUN go install .

# Save licenses, etc.
RUN go run . save . --save_path /THIRD_PARTY_NOTICES

# Make a minimal image.
FROM gcr.io/distroless/base

COPY --from=build /go/bin/go-licenses /
COPY --from=build /THIRD_PARTY_NOTICES /THIRD_PARTY_NOTICES

ENTRYPOINT ["/go-licenses"]
