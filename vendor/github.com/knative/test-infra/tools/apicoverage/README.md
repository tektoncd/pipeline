# API Coverage Tool
This tool is designed to show the field level coverage exercised by the conformance tests.

## Read from GCS
This tool reads the logs from the latest continous build of knative/serving. The logs have the information of which CRD objects are being created and which fields are being set for the testing.
It uses the service account passed in or by default will use the GOOGLE_APPLICATION_CREDENTIALS variable to get the logs. 

## Creating Output
This tool creates an output xml in the prow artifacts directory. The prow artifacts directory is passed in or by default will use `./artifacts` directory.

This output xml will be read by testgrid and displayed on the [dashboard](https://testgrid.knative.dev/knative-serving#api-coverage).

## Prow Job
There is a daily prow job that triggers this tool that is run at 01:05 AM PST. This tool will then generate the output xml which is then displayed in the testgrid dashboard.
