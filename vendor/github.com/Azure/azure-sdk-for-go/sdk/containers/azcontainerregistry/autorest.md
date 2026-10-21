# Autorest config for Azure Container Registry Go client

> see https://aka.ms/autorest

## Configuration

```yaml
input-file: https://github.com/Azure/azure-rest-api-specs/blob/c8d9a26a2857828e095903efa72512cf3a76c15d/specification/containerregistry/data-plane/Azure.ContainerRegistry/stable/2021-07-01/containerregistry.json
license-header: MICROSOFT_MIT_NO_VERSION
go: true
clear-output-folder: false
export-clients: true
openapi-type: "data-plane"
output-folder: ../azcontainerregistry
use: "@autorest/go@4.0.0-preview.60"
honor-body-placement: true
remove-unreferenced-types: true
module-name: sdk/containers/azcontainerregistry
module: github.com/Azure/azure-sdk-for-go/$(module-name)
inject-spans: true
```

## Customizations

See the [AutoRest samples](https://github.com/Azure/autorest/tree/master/Samples/3b-custom-transformations)
for more about how we're customizing things.

### Remove response for "ContainerRegistry_DeleteRepository" operation

so that the generated code doesn't return a response for the deleted repository operation.

```yaml
directive:
  - from: swagger-document
    where: $["paths"]["/acr/v1/{name}"]
    transform: >
      delete $.delete["responses"]["202"].schema
```

### Remove response for "ContainerRegistryBlob_DeleteBlob" operation

so that the generated code doesn't return a response for the deleted blob operation.

```yaml
directive:
  - from: swagger-document
    where: $["paths"]["/v2/{name}/blobs/{digest}"]
    transform: >
      delete $.delete["responses"]["202"].schema
```

### Remove "Authentication_GetAcrAccessTokenFromLogin" operation

as the service team discourage using username/password to authenticate.

```yaml
directive:
  - from: swagger-document
    where: $["paths"]["/oauth2/token"]
    transform: >
      delete $.get
```

### Remove "ContainerRegistry_CheckDockerV2Support" operation

```yaml
directive:
  - from: swagger-document
    where: $["paths"]["/v2/"]
    transform: >
      delete $.get
```

### Remove "definitions.TagAttributesBase.properties.signed"

as we don't have customer scenario using it.

```yaml
directive:
  - from: swagger-document
    where: $.definitions.TagAttributesBase
    transform: >
      delete $.properties.signed
```

### Add "definitions.ManifestAttributesBase.properties.mediaType"

```yaml
directive:
  - from: swagger-document
    where: $.definitions.ManifestAttributesBase
    transform: >
      $.properties["mediaType"] = {
        "type": "string",
        "description": "Media type for this Manifest"
      }
```

### Change "parameters.ApiVersionParameter.required" to true

so that the API version could be removed from client parameter.

```yaml
directive:
  - from: swagger-document
    where: $.parameters.ApiVersionParameter
    transform: >
      $.required = true
```

### Take stream as manifest body

```yaml
directive:
  from: swagger-document
  where: $.parameters.ManifestBody
  transform: >
    $.schema = {
      "type": "string",
      "format": "binary"
    }
```

### Change list order by param to enum

```yaml
directive:
  - from: containerregistry.json
    where: $.paths["/acr/v1/{name}/_tags"].get
    transform: >
      $.parameters.splice(3, 1);
      $.parameters.push({
        "name": "orderby",
        "x-ms-client-name": "OrderBy",
        "in": "query",
        "required": false,
        "x-ms-parameter-location": "method",
        "type": "string",
        "description": "Sort options for ordering tags in a collection.",
        "enum": [
          "none",
          "timedesc",
          "timeasc"
        ],
        "x-ms-enum": {
          "name": "ArtifactTagOrderBy",
          "values": [
            {
              "value": "none",
              "name": "None",
              "description": "Do not provide an orderby value in the request."
            },
            {
              "value": "timedesc",
              "name": "LastUpdatedOnDescending",
              "description": "Order tags by LastUpdatedOn field, from most recently updated to least recently updated."
            },
            {
              "value": "timeasc",
              "name": "LastUpdatedOnAscending",
              "description": "Order tags by LastUpdatedOn field, from least recently updated to most recently updated."
            }
          ]
        }
      });
  - from: containerregistry.json
    where: $.paths["/acr/v1/{name}/_manifests"]
    transform: >
      $.get.parameters.splice(3, 1);
      $.get.parameters.push({
        "name": "orderby",
        "x-ms-client-name": "OrderBy",
        "in": "query",
        "required": false,
        "x-ms-parameter-location": "method",
        "type": "string",
        "description": "Sort options for ordering manifests in a collection.",
        "enum": [
          "none",
          "timedesc",
          "timeasc"
        ],
        "x-ms-enum": {
          "name": "ArtifactManifestOrderBy",
          "values": [
            {
              "value": "none",
              "name": "None",
              "description": "Do not provide an orderby value in the request."
            },
            {
              "value": "timedesc",
              "name": "LastUpdatedOnDescending",
              "description": "Order manifests by LastUpdatedOn field, from most recently updated to least recently updated."
            },
            {
              "value": "timeasc",
              "name": "LastUpdatedOnAscending",
              "description": "Order manifest by LastUpdatedOn field, from least recently updated to most recently updated."
            }
          ]
        }
      });
```

### Rename paged operations from Get* to List*

```yaml
directive:
  - rename-operation:
      from: ContainerRegistry_GetManifests
      to: ContainerRegistry_ListManifests
  - rename-operation:
      from: ContainerRegistry_GetRepositories
      to: ContainerRegistry_ListRepositories
  - rename-operation:
      from: ContainerRegistry_GetTags
      to: ContainerRegistry_ListTags
```

### Change ContainerRegistry_CreateManifest behaviour

```yaml
directive:
  from: swagger-document
  where: $.paths["/v2/{name}/manifests/{reference}"].put
  transform: >
    $.consumes.push("application/vnd.oci.image.manifest.v1+json");
    delete $.responses["201"].schema;
```

### Change ContainerRegistry_GetManifest behaviour

```yaml
directive:
  from: swagger-document
  where: $.paths["/v2/{name}/manifests/{reference}"].get.responses["200"]
  transform: >
    $.schema = {
      type: "string",
      format: "file"
    };
    $.headers = {
      "Docker-Content-Digest": {
        "type": "string",
        "description": "Digest of the targeted content for the request."
      }
    };
```

### Remove generated constructors

```yaml
directive:
  - from: 
      - authentication_client.go
      - client.go
      - blob_client.go
    where: $
    transform: return $.replace(/(?:\/\/.*\s)+func New.+Client.+\{\s(?:.+\s)+\}\s/, "");
```

### Rename operations

```yaml
directive:
  - rename-operation:
      from: ContainerRegistry_GetProperties
      to: ContainerRegistry_GetRepositoryProperties
  - rename-operation:
      from: ContainerRegistry_UpdateProperties
      to: ContainerRegistry_UpdateRepositoryProperties
  - rename-operation:
      from: ContainerRegistry_UpdateTagAttributes
      to: ContainerRegistry_UpdateTagProperties
  - rename-operation:
      from: ContainerRegistry_CreateManifest
      to: ContainerRegistry_UploadManifest
```

### Rename parameter name

```yaml
directive:
  from: swagger-document
  where: $.parameters
  transform: >
    $.DigestReference["x-ms-client-name"] = "digest";
    $.TagReference["x-ms-client-name"] = "tag";
```

### Add 202 response to ContainerRegistryBlob_MountBlob

```yaml
directive:
  from: swagger-document
  where: $.paths["/v2/{name}/blobs/uploads/"]
  transform: >
    $.post["responses"]["202"] = $.post["responses"]["201"];
```

### Extract and add endpoint for nextLink

```yaml
directive:
  - from:
      - client.go
    where: $
    transform: return $.replaceAll(/result\.Link = &val/g, "val = runtime.JoinPaths(client.endpoint, extractNextLink(val))\n\t\tresult.Link = &val");
```

### Rename all Acr to ACR

```yaml
directive:
  - from:
      - "*.go"
    where: $
    transform: return $.replaceAll(/Acr/g, "ACR");
```

### Rename TagAttributesBase, ManifestAttributesBase, TagAttributeBases, Repositories, AcrManifests and QueryNum

```yaml
directive:
  - from: containerregistry.json
    where: $.definitions
    transform: >
      $.TagAttributesBase["x-ms-client-name"] = "TagAttributes";
  - from: containerregistry.json
    where: $.definitions
    transform: >
      $.ManifestAttributesBase["x-ms-client-name"] = "ManifestAttributes";
  - from: containerregistry.json
    where: $.definitions.TagList
    transform: >
      delete $.properties.tags["x-ms-client-name"];
  - from: containerregistry.json
    where: $.definitions.Repositories
    transform: >
      $.properties.repositories["x-ms-client-name"] = "Names";
  - from: containerregistry.json
    where: $.definitions
    transform: >
      $.AcrManifests["x-ms-client-name"] = "Manifests";
  - from: containerregistry.json
    where: $.definitions.AcrManifests
    transform: >
      $.properties.manifests["x-ms-client-name"] = "Attributes";
  - from: containerregistry.json
    where: $.parameters
    transform: >
      $.QueryNum["x-ms-client-name"] = "MaxNum";
```

### Rename binary request param and response property

```yaml
directive:
  - from: containerregistry.json
    where: $.parameters
    transform: >
      $.RawData["x-ms-client-name"] = "chunkData";
      $.RawDataOptional["x-ms-client-name"] = "blobData";
      $.ManifestBody["x-ms-client-name"] = "manifestData";
  - from:
      - blob_client.go
    where: $
    transform: return $.replace(/BlobClientGetBlobResponse\{Body/, "BlobClientGetBlobResponse{BlobData").replace(/BlobClientGetChunkResponse\{Body/, "BlobClientGetChunkResponse{ChunkData");
  - from:
      - client.go
    where: $
    transform: return $.replace(/ClientGetManifestResponse\{Body/, "ClientGetManifestResponse{ManifestData");
  - from:
      - response_types.go
    where: $
    transform: return $.replace(/Body io\.ReadCloser/, "BlobData io.ReadCloser").replace(/Body io\.ReadCloser/, "ChunkData io.ReadCloser").replace(/Body io\.ReadCloser/, "ManifestData io.ReadCloser");
```

### Hide original UploadChunk and CompleteUpload method
```yaml
directive:
  - from: containerregistry.json
    where: $.paths["/{nextBlobUuidLink}"]
    transform: >
      $.put.parameters.splice(1,1);
  - from:
      - blob_client.go
    where: $
    transform: return $.replaceAll(/ UploadChunk/g, " uploadChunk").replace(/\.UploadChunk/, ".uploadChunk").replaceAll(/ CompleteUpload/g, " completeUpload").replace(/\.CompleteUpload/, ".completeUpload");
```

### Add content-range parameters to upload chunk

```yaml
directive:
  - from: swagger-document
    where: $.paths["/{nextBlobUuidLink}"].patch
    transform: >
        $.parameters.push({
            "name": "Content-Range",
            "in": "header",
            "type": "string",
            "description": "Range of bytes identifying the desired block of content represented by the body. Start must the end offset retrieved via status check plus one. Note that this is a non-standard use of the Content-Range header."
        });
  - from:
      - blob_client.go
      - options.go
    where: $
    transform: return $.replaceAll(/BlobClientUploadChunkOptions/g, "blobClientUploadChunkOptions").replace(/BlobClient\.UploadChunk/, "BlobClient.uploadChunk");
```

### Add description for ArtifactOperatingSystem

```yaml
directive:
  - from: swagger-document
    where: $.definitions
    transform: >
        $.ArtifactOperatingSystem.description = "The artifact platform's operating system.";
```

### Add description for RefreshToken and AccessToken

```yaml
directive:
  - from: swagger-document
    where: $.definitions
    transform: >
        $.RefreshToken.description = "The ACR refresh token response.";
  - from: swagger-document
    where: $.definitions
    transform: >
      $.AccessToken.description = "The ACR access token response.";
```

### Remove useless Marshal method

```yaml
directive:
  - from:
      - models_serde.go
    where: $
    transform: >
      return $
      .replace(/\/\/ MarshalJSON.*TagList[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*TagAttributes[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*Repositories[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*Manifests[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*ManifestAttributes[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*ContainerRepositoryProperties[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*ArtifactTagProperties[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*ArtifactManifestProperties[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*ArtifactManifestPlatform[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*acrRefreshToken[^}]*}\n/g, "")
      .replace(/\/\/ MarshalJSON.*acrAccessToken[^}]*}\n/g, "")
```