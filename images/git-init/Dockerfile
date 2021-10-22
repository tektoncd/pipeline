FROM alpine:3.14

RUN addgroup -S -g 65532 nonroot && adduser -S -u 65532 nonroot -G nonroot

RUN apk add --update git git-lfs openssh-client \
    && apk update \
    && apk upgrade
