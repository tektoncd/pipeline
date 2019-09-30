# Do not edit! This file was generated via Makefile
FROM registry.access.redhat.com/ubi8/ubi:latest

# NOTE(chmou): We use dollar here so that envsubst don't get confused and expand
# our local PATH.
ENV HOME=/ko-app PATH=${HOME}:${PATH}
RUN yum install --disableplugin=subscription-manager -y git openssh-clients

COPY git-init ${HOME}/git-init.orig
COPY uidwrapper ${HOME}/git-init

RUN chgrp -R 0 ${HOME} && \
    chmod -R g=u ${HOME} /etc/passwd

ENTRYPOINT ["/ko-app/git-init"]

# Local Variables:
# mode: dockerfile
# End:
