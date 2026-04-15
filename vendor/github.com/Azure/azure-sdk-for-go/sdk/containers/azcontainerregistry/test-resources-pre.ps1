# Copyright (c) Microsoft Corporation. All rights reserved.
# Licensed under the MIT License.

# IMPORTANT: Do not invoke this file directly. Please instead run eng/common/TestResources/New-TestResources.ps1 from the repository root.

param (
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string] $SubscriptionId,

    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string] $TenantId,

    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$')]
    [string] $TestApplicationId,

    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string] $Environment,

    [Parameter()]
    [switch] $CI = ($null -ne $env:SYSTEM_TEAMPROJECTID),

    # Captures any arguments from eng/New-TestResources.ps1 not declared here (no parameter errors).
    [Parameter(ValueFromRemainingArguments = $true)]
    $RemainingArguments
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

if ($CI) {
    az cloud set -n $Environment
    az login --federated-token $env:ARM_OIDC_TOKEN --service-principal -t $TenantId -u $TestApplicationId
    if ($LASTEXITCODE) { exit $LASTEXITCODE }
    az account set --subscription $SubscriptionId
    if ($LASTEXITCODE) { exit $LASTEXITCODE }
}
