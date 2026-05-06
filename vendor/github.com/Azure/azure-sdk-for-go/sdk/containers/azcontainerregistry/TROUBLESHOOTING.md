# Troubleshoot Azure Container Registry client library issues

This troubleshooting guide contains instructions to diagnose frequently encountered issues while using the Azure Container Registry client library for Go.

## General Troubleshooting

### Error Handling

All methods which send HTTP requests return `*azcore.ResponseError` when these requests fail. `ResponseError` has error details and the raw response from Container Registry.

```go
import "github.com/Azure/azure-sdk-for-go/sdk/azcore"

resp, err := client.GetRepositoryProperties(ctx, "library/hello-world", nil)
if err != nil {
	var httpErr *azcore.ResponseError
	if errors.As(err, &httpErr) {
		// TODO: investigate httpErr
	} else {
		// TODO: not an HTTP error
	}
}
```

### Logging

This module uses the logging implementation in `azcore`. To turn on logging for all Azure SDK modules, set `AZURE_SDK_GO_LOGGING` to `all`. By default, the logger writes to stderr. Use the `azcore/log` package to control log output. For example, logging only HTTP request and response events, and printing them to stdout:

```go
import azlog "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"

// Print log events to stdout
azlog.SetListener(func(cls azlog.Event, msg string) {
	fmt.Println(msg)
})

// Includes only requests and responses in credential logs
azlog.SetEvents(azlog.EventRequest, azlog.EventResponse)
```

### Accessing `http.Response`

You can access the raw `*http.Response` returned by Container Registry using the `runtime.WithCaptureResponse` method and a context passed to any client method.

```go
import "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"

var response *http.Response
ctx := runtime.WithCaptureResponse(context.TODO(), &response)
_, err = client.GetRepositoryProperties(ctx, "library/hello-world", nil)
if err != nil {
	// TODO: handle error
}
// TODO: do something with response
```

## Troubleshooting authentication errors

### HTTP 401 Errors

HTTP 401 errors indicates problems authenticating. Check exception message or logs for more information.

#### ARM access token is disabled

You may see error similar to the one below, it indicates authentication with ARM access token was disabled on accessed Container Registry resource.
Refer to [ACR CLI reference](https://learn.microsoft.com/cli/azure/acr/config/authentication-as-arm?view=azure-cli-latest) for information on how to
check and configure authentication with ARM tokens.

```text
--------------------------------------------------------------------------------
RESPONSE 401: 401 Unauthorized
ERROR CODE UNAVAILABLE
--------------------------------------------------------------------------------
{
  "errors": [
    {
      "code": "UNAUTHORIZED",
      "message": "arm aad token disallowed"
    }
  ]
}
--------------------------------------------------------------------------------
```

#### Anonymous access issues
You may see error similar to the one below, it indicates an attempt to perform operation that requires authentication without credentials.

```text
--------------------------------------------------------------------------------
RESPONSE 401: 401 Unauthorized
ERROR CODE UNAVAILABLE
--------------------------------------------------------------------------------
{
  "errors": [
    {
      "code": "UNAUTHORIZED",
      "message": "authentication required, visit https://aka.ms/acr/authorization for more information."
    }
  ]
}
--------------------------------------------------------------------------------
```

Unauthorized access can only be enabled for read (pull) operations such as listing repositories, getting properties or tags.
Refer to [Anonymous pull access](https://docs.microsoft.com/azure/container-registry/anonymous-pull-access) to learn about anonymous access limitation.

### HTTP 403 Errors

HTTP 403 errors indicate the user is not authorized to perform a specific operation in Azure Container Registry.

#### Insufficient permissions

If you see an error similar to the one below, it means that the provided credentials does not have permissions to access the registry.
```text
--------------------------------------------------------------------------------
RESPONSE 403: 403 Forbidden
ERROR CODE UNAVAILABLE
--------------------------------------------------------------------------------
{
  "errors": [
    {
      "code": "DENIED",
      "message": "retrieving permissions failed"
    }
  ]
}
--------------------------------------------------------------------------------
```

1. Check that the application or user that is making the request has sufficient permissions.
   Check [Troubleshoot registry login](https://docs.microsoft.com/azure/container-registry/container-registry-troubleshoot-login) for possible solutions.
2. If the user or application is granted sufficient privileges to query the workspace, make sure you are
   authenticating as that user/application. See the [azidentity](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) documentation for more information.

#### Network access issues

You may see an error similar to the one below, it indicates that public access to Azure Container registry is disabled or restricted.
Refer to [Troubleshoot network issues with registry](https://docs.microsoft.com/azure/container-registry/container-registry-troubleshoot-access) for more information.
```text
--------------------------------------------------------------------------------
RESPONSE 403: 403 Forbidden
ERROR CODE UNAVAILABLE
--------------------------------------------------------------------------------
{
  "errors": [
    {
      "code": "DENIED",
      "message": "client with IP <> is not allowed access. Refer https://aka.ms/acr/firewall to grant access."
    }
  ]
}
--------------------------------------------------------------------------------
```

## Service errors

When working with `azcontainerregistry.Client` and `azcontainerregistry.BlobClient` you may get `*azcore.ResponseError` with
message containing additional information and [Docker error code](https://docs.docker.com/registry/spec/api/#errors-2).

### Getting BLOB_UPLOAD_INVALID

In rare cases, transient error (such as connection reset) can happen during upload chunk. You may see an error similar to the one below. In this case upload should to be restarted from the beginning.
```text
--------------------------------------------------------------------------------
RESPONSE 404: 404 Not Found Error
ERROR CODE UNAVAILABLE
--------------------------------------------------------------------------------
{
  "errors": [
    {
      "code": "BLOB_UPLOAD_INVALID",
      "message": "blob upload invalid"
    }
  ]
}
--------------------------------------------------------------------------------
```