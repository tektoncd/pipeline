FROM alpine:3.14 as source
RUN addgroup -S -g 65532 nonroot && adduser -S -u 65532 nonroot -G nonroot

FROM gcr.io/distroless/static:latest
COPY --from=source /etc/passwd /etc/passwd
COPY --from=source /etc/group /etc/group
