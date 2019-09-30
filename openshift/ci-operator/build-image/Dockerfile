# Dockerfile to bootstrap build and test in openshift-ci

FROM openshift/origin-release:golang-1.10

# Add kubernetes repository
ADD openshift/ci-operator/build-image/kubernetes.repo /etc/yum.repos.d/

RUN yum install -y kubectl ansible

# Allow runtime users to add entries to /etc/passwd
RUN chmod g+rw /etc/passwd
