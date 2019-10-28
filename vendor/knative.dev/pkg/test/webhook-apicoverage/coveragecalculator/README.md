# Coverage Calculator

`coveragecalculator` package contains types and helper methods pertaining to
coverage calculation.

[TypeCoverage](coveragedata.go) is a type to represent coverage data for a
particular API object type. This is the wire contract between the webhook server
running inside the K8s cluster and any client using the API-Coverage tool. All
API calls into the webhook-server would return response containing this object
to represent coverage data.

[IgnoredFields](ignorefields.go) type provides ability for individual repos to
specify fields that they would like the API Coverage tool to ignore for coverage
calculation. Individual repos are expected to provide a .yaml file providing
fields that they would like to ignore and use helper method
`ReadFromFile(filePath)` to read and intialize this type. `FieldIgnored()` can
then be called by providing `packageName`, `typeName` and `FieldName` to check
if the field needs to be ignored.

[CalculateCoverage](calculator.go) method provides a capability to calculate
coverage values. This method takes an array of [TypeCoverage](coveragedata.go)
and iterates over them to aggreage coverage values. The aggregate result is
encapsulated inside [CoverageValues](calculator.go) and returned.
