FROM alpine:latest

RUN apk add --update git openssh-client jq \
    && apk update \
    && apk upgrade
