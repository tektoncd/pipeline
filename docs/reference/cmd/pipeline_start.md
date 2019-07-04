# pipeline start additional content

## Description

To start a pipeline, you will need to pass the following:

- Resources
- Parameters, at least those that have no default value

## Examples

To run a Pipeline that has one git resource and no parameter.

```shell
$ tkn pipeline start --resource source=samples-git
```

To run a Pipeline that has one git resource, one image resource and
two parameters (foo and bar)

```shell
$ tkn pipeline start --resource source=samples-git \
	--resource image=my-image \
	--param foo=yay \
	--param bar=10
```
