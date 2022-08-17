# combine manifest lists

This tool combines two or more Docker manifest lists ("multi-arch images") into one that provides all the platforms supported by all the specified base image manifest lists.

It fails if any images provide the same platforms, or if any isn't a manifest list.

# demo

```
$ go run ./ \
    ghcr.io/distroless/static \
    mcr.microsoft.com/windows/nanoserver:ltsc2022 \
    mcr.microsoft.com/windows/nanoserver:ltsc2019 \
    gcr.io/MY_PROJECT/combined
```

This combines a [distroless](https://github.com/distroless/static) base image providing Linux platform support, with two image providing different versions of a Windows base image.

The general form of the command args is:

```
combine src1 src2 [src...] dst
```

(combine two or more source manifest lists and push to the `dst` reference).

After running the script, you can check the image's platforms using [`crane`](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md):

```
$ crane manifest gcr.io/MY_PROJECT/combined | jq '.manifests[].platform'
{
  "architecture": "386",
  "os": "linux"
}
{
  "architecture": "amd64",
  "os": "linux"
}
{
  "architecture": "arm",
  "os": "linux",
  "variant": "v6"
}
{
  "architecture": "arm",
  "os": "linux",
  "variant": "v7"
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
  "architecture": "riscv64",
  "os": "linux"
}
{
  "architecture": "s390x",
  "os": "linux"
}
{
  "architecture": "amd64",
  "os": "windows",
  "os.version": "10.0.20348.887"
}
{
  "architecture": "amd64",
  "os": "windows",
  "os.version": "10.0.17763.3287"
}
```

The result is an image that provides support for all the provided base image platforms.
If any manifest lists provide duplicate support for the same platform, the script will fail.

This image is intended to be suitable as a base image used with `ko` to provide multi-arch _and multi-OS_ support for a Go application.
