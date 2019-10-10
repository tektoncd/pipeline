# View

This package contains types and helper methods that repos can use to display API
Coverage results.

[DisplayRules](rule.go) provides a mechanism for repos to define their own
display rules. DisplayHelper methods can use these rules to define how to
display results.

`GetHTMLDisplay()` is a utility method that can be used by repos to get a
HTML(JSON) like textual display of API Coverage. This method takes an array of
[TypeCoverage](../coveragecalculator/coveragedata.go) and
[DisplayRules](rule.go) object and returns a string representing its coverage in
the color coded format inside a HTML page:

```
Package: <PackageName>
Type: <TypeName>
{
    <FieldName> <Ignored>/<Coverage:TrueorFalse> [Values]
    ....
    ....
    ....
}
```

`GetHTMLCoverageValuesDisplay()` is a utility method that can be used by repos
to produce coverage values display. The method takes as input
[CoverageValue](../coveragecalculator/calculator.go) and produces a display in
the format inside a HTML page:

```
CoverageValues:

Total Fields:  <Number of total fields>
Covered Fields: <Number of fields covered>
Ignored Fields: <Number of fields ignored>
Coverage Percentage: <Percentage value of coverage>
```

`GetCoveragePercentageXMLDisplay()` is a utility method that can be used by
repos to produce coverage percentage for each resource in a Junit XML results
file. The method takes
[CoveragePercentages](../coveragecalculator/calculator.go) as input and produces
a Junit result file format.
