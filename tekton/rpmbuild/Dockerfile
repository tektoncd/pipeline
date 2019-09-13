FROM fedora:latest
RUN dnf update -y && dnf -y install python rpm-build copr-cli && rm -rf /var/lib/rpm/cache && dnf -y clean all
