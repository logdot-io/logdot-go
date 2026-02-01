[CmdletBinding()]
param(
    [switch]$Bump
)

$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot

try {
    # Get the latest version tag
    $latestTag = git describe --tags --abbrev=0 --match "v*" 2>$null
    if (-not $latestTag) { $latestTag = "v0.0.0" }
    $currentVersion = $latestTag.TrimStart('v')

    if ($Bump) {
        $parts = $currentVersion.Split('.')
        $newPatch = [int]$parts[2] + 1
        $newVersion = "$($parts[0]).$($parts[1]).$newPatch"
        $tag = "v${newVersion}"

        Write-Host "Bumping version: v${currentVersion} -> ${tag}"
    } else {
        $newVersion = $currentVersion
        $tag = "v${newVersion}"
        Write-Host "Publishing ${tag}..."
    }

    Write-Host "Running tests..."
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Tests failed" }

    Write-Host "Running vet..."
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "Vet failed" }

    if ($Bump) {
        Write-Host "Creating tag ${tag}..."
        git tag -a $tag -m "Release ${tag}"
        if ($LASTEXITCODE -ne 0) { throw "git tag failed" }
    }

    Write-Host "Pushing tag ${tag}..."
    git push origin $tag
    if ($LASTEXITCODE -ne 0) { throw "git push failed" }

    Write-Host "Successfully published github.com/logdot-io/logdot-go ${tag}"
    Write-Host "Module will be available on pkg.go.dev shortly."
}
finally {
    Pop-Location
}
