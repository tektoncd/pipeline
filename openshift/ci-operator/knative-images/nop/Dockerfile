# Do not edit! This file was generated via Makefile
FROM registry.access.redhat.com/ubi8/ubi:latest

# NOTE(chmou): *BEWARE YOU ARE ENTERING IN A WORLD OF UGLY HACK#.
# Sidecar is currently broken with a nop image that's not based on google's
# distroless. We want to mimic as much as possible the distroless image but
# still be based on a UBI, it's not pretty but let's hope we can remove this when
# we get https://github.com/tektoncd/pipeline/issues/1347
RUN rm -f /usr/bin/* /usr/sbin/* /bin/* /sbin/*

ADD nop /ko-app/nop
ENTRYPOINT ["/ko-app/nop"]
