# combine manifest lists

This tool combines two Docker manifest lists ("multi-arch images") into one that provides all the platforms supported by both manifest lists.

It fails if both images provide the same platforms, or if either isn't a manifest list.

# demo

```
$ go run ./ \
    gcr.io/distroless/static:nonroot \
    mcr.microsoft.com/windows/nanoserver:1809 \
    gcr.io/imjasonh/combined
```

This combines the [distroless](https://github.com/googlecontainertools/distroless) image providing linux platform support with an image providing Windows support.

The general form of the command args is:

```
combine src1 src2 dst
```

(combine two source manifest lists and push to the `dst` reference).

After running the script, you can check the image's platforms using [`crane`](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md):

```
$ crane manifest gcr.io/imjasonh/combined | jq '.manifests[].platform'
{
  "architecture": "amd64",
  "os": "linux"
}
{
  "architecture": "arm",
  "os": "linux"
}
{
  "architecture": "arm64",
  "os": "linux"
}
{
  "architecture": "ppc64le",
  "os": "linux"
}
{
  "architecture": "s390x",
  "os": "linux"
}
{
  "architecture": "amd64",
  "os": "windows",
  "os.version": "10.0.17763.1935"
}
{
  "architecture": "arm",
  "os": "windows",
  "os.version": "10.0.17763.1935"
}
```

The result is an image that provides support for both.
If both manifest lists provide support for the same platform, the script will fail.

This image is intended to be suitable as a base image used with `ko` to provide multi-arch _and multi-OS_ support for a Go application.
