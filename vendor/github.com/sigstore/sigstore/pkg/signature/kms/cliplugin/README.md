# CLI Plugin

This is a package and module for using custom KMS plugins as separate executables.
It is intended to be used by cosign, but you may use this in your own programs that import sigstore.

## Design

We follow [kubectl's style](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/#writing-kubectl-plugins) of plugins. Any language that can create CLI programs can be used to make a plugin.

### Usage

Plugins are separate programs on your system's PATH, named in the scheme `sigstore-kms-[name]`, like `sigstore-kms-my-hsm`. They can be invoked with cosign like `cosign [sub-command] --key "my-hsm://my-key-id" ...
`

### Protocol

The main program will invoke the program with these specifications:

* stdin
  * Data to be signed or verified.
* arg 1
  * A number identifying the version of this protocol.
    * In the future we may change this protocol, either the encoding or the formatting of argument and return values.
    Protocol changes will result in a major-version bump for the library.
    When changing the protocol, new versions of sigstore will not maintain backwards compatibility with
    previous protocol versions. If a plugin author wishes, they may branch their plugin program’s behaviour
    to be compatible with multiple versions of the protocol, or multiple major versions of sigstore and cosign.
* arg 2
  * JSON of initialization options and method arguments.
* stdout
  * JSON of method return values.

See [./common/interface.go](./common/interface.go) and [./common/interface_test.go](./common/interface_test.go) for the full JSON schema.

The plugin program must first exit before sigstore begins parsing responses.

#### Error Handling

The plugin program’s stderr will be redirected to the main program’s stderr. This way, the main program may also see the plugin program’s debug messages.

Plugin authors may return errors with `PluginResp.ErrorMessage`, but the plugin's exit status will be ignored.

### Implementation

Plugin authors must implement the `kms.SignerVerifier` interface methods in their chosen language. Each method will invoke your program once, and the response will be parsed from stdout.

`PluginClient.CryptoSigner()` will return object that is a wrapper around `PluginClient`, so plugin authors need not do a full implementation of `SignerVerifier()`.

Exit status is ignored. Your program's stderr will be redirected to the main program, and errors you wish to return must be serialized in `PluginResp.ErrorMessage` in stdout.

For authors using Go, we vend some helper functions to help you get started. See [handler](./handler/README.md)

## Development

Changes to the `SignerVerifier` interface are to be handled in [./signer.go's](./signer.go) `PluginClient` and [./handler/dispatch.go's](./handler/dispatch.go) `Dispatch()`.

### Adding New Methods or Method Options

Adding new methods or options are *not* necessarily breaking changes to the schemas, so we may consider these to be minor version increments, both to the protocol version and the sigstore version.

### Removing Methods

Removing methods, or altering their signatures will break the schemas and will require major version increments, both to the protocol version and the sigstore version.

### Example Plugin

We have an example plugin in [test/cliplugin/localkms](../../../../test/cliplugin/localkms).

1. Compile cosign and the plugin

    ```shell
    go build -C cosign/cmd/cosign -o `pwd`/cosign-cli
    go build -C sigstore/test/cliplugin/localkms -o `pwd`/sigstore-kms-localkms
    ```

2. Sign some data

    With our example, you need to first create the key.

    ```shell
    export PATH="$PATH:`pwd`"
    cosign-cli generate-key-pair --kms localkms://`pwd`/key.pem
    cat cosign.pub
    ```

    Sign some data.

    ```shell
    export PATH="$PATH:`pwd`"
    echo "my-data" > blob.txt
    cosign-cli sign-blob --tlog-upload=false --key localkms://`pwd`/key.pem blob.txt
    ```

### Testing

Unit tests against an example plugin program are in [./signer_program_test.go](./signer_program_test.go).
Compile the plugin and invoke unit tests with

```shell
make test-signer-program
```

Or invoke the unit tests with your own pre-compiled plugin program like


```shell
export PATH=$PATH:[folder containing plugin program]
go test -C ./pkg/signature/kms/cliplugin -v -tags=signer_program ./... -key-resource-id [my-kms]://[my key ref]
```
