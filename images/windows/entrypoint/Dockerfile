ARG BASE_IMAGE=mcr.microsoft.com/windows/nanoserver:1809

FROM golang:1.16.13 AS builder

COPY . c:/gopath/src/github.com/tektoncd/pipeline

WORKDIR c:/gopath/src/github.com/tektoncd/pipeline

RUN go build -o c:/gopath/bin/entrypoint ./cmd/entrypoint

FROM ${BASE_IMAGE}

COPY --from=builder c:/gopath/bin/entrypoint c:/ko-app/entrypoint

# Copy kodata into the image
# NOTE: The COPY instruction does not follow symbolic links
COPY ./.git/HEAD ./LICENSE c:/ProgramData/tektoncd/pipeline/data/

COPY ./.git/refs c:/ProgramData/tektoncd/pipeline/data/refs/

COPY ./third_party c:/ProgramData/tektoncd/pipeline/data/third-party/

ENTRYPOINT ["c:/ko-app/entrypoint"]
