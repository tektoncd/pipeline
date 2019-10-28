This is used by the Mako stub sidecar in test/mako/stub-sidecar. Once that code
is removed, this can also be removed.

It's here because Mako's quickstore service proto is under an internal
directory, and Go refuses to import packages marked internal.
