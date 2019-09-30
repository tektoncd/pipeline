# Do not edit! This file was generated via Makefile
FROM registry.access.redhat.com/ubi8/ubi:latest

# NOTE(chmou): We use dollar here so that envsubst don't get confused and expand
# our local PATH.
ENV HOME=/ko-app PATH=${HOME}:${PATH}
RUN yum install --disableplugin=subscription-manager -y git openssh-clients

COPY creds-init ${HOME}/creds-init.orig
COPY uidwrapper ${HOME}/creds-init

RUN chgrp -R 0 ${HOME} && \
    chmod -R g=u ${HOME} /etc/passwd

ENTRYPOINT ["/ko-app/creds-init"]

# Local Variables:
# mode: dockerfile
# End:
