# Logs Credential Filter (TEP-0125)

Often it is the case that secret values will sneak into the ouput logs of your pipelines. You can either handle this manually by avoiding to print them in the output or use the feature for filtering secret values from the output log. To enable this feature set the `enable-logging-credentials-filter` value in the `feature-flags` config map to `"true"` (see [Customizing the Pipelines Controller behavior](install.md#customizing-the-pipelines-controller-behavior)).

The tekton controller looks for secret refs in volumes and env and also for CSI volumes referencing secrets in the definition of the pipeline pod. It saves the information in form of the names of environment variables and paths to files with secrets. The pod can then read those values and redact them.

This design does not transmit actual secrets and the pod does not need access to the API sever to retrieve them. It tells the pipeline pod only the places where it can find secrets attached to it. And those are the secrets that will be redacted. Because only those can be used in the corresponding pipeline step.

The credential filter will redact values contained in those secrets from the output log stream and replace them with `[REDACTED:<secret-name>]`.

## Secret Detection Logic

While creating the pod for the pipeline step, the controller will try to detect all secrets attached to the pod. Usually secrets are attached to the pod either by setting environment variables or mounting files into it. These detected secret locations are then transmitted to the pipeline pod for credential filtering.

### Secret Locations

The secret locations detected by the controller are transmitted to the entrypoint as environment variable in json format with the following format:

```json
{
  "environmentVariables": ["ENV1", "ENV2"],
  "files": ["/path/to/secre1", "/path/to/secret2"]
}
```

The entrypoint can then read the environment variables and file contents and redact them from the output log stream.

### Secrets stored in Environment Variables

The controller detects secrets stored as environment variables from the following pod syntax:

```yaml
env:
- name: MY_SECRET_VALUE_IN_ENV
  valueFrom:
    secretKeyRef:
      name: my-k8s-secret
      key: secret_value_key
```

The secret key reference is the trigger to detect an environment variable that contains a secret and that needs to be redacted.

### Secrets mounted as Files 

Secrets can also be mounted into pods via files in different ways. Currently the following pod syntax is supported in the secret detection logic.

```yaml
volumes:
- name: secret-volume
  secret:
    secretName: my-k8s-secret
```

```yaml
volumes:
- name: secret-volume-csi
  csi:
    driver: secrets-store.csi.k8s.io
    readOnly: true
    volumeAttributes:
      secretProviderClass: secret-provider-class
```

Currently classic secret volumes and csi secret volumes with the driver `secrets-store.csi.k8s.io` are supported. In both cases the volume directories or items in that volume will be added to the detected secret locations.