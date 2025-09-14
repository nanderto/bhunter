# Bitbucket Hunter Build Script
# Usage: .\build-release.ps1 [version]
# Example: .\build-release.ps1 v2.0.1

param(
    [Parameter(Position=0)]
    [string]$Version = "dev"
)

Write-Host "Building Bitbucket Hunter release..." -ForegroundColor Green
Write-Host "Version: $Version" -ForegroundColor Yellow

# Get build info
$commit = git rev-parse --short HEAD
$date = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"

Write-Host "Commit: $commit" -ForegroundColor Yellow  
Write-Host "Build Date: $date" -ForegroundColor Yellow

# Ensure release directory exists
if (!(Test-Path "release")) {
    New-Item -ItemType Directory -Path "release"
    Write-Host "Created release directory" -ForegroundColor Green
}

# Build standard version
Write-Host "`nBuilding standard version..." -ForegroundColor Cyan
go build -ldflags "-X main.version=$Version -X main.commit=$commit -X main.date=$date" -o release/bhunter.exe
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Standard build completed" -ForegroundColor Green
} else {
    Write-Host "✗ Standard build failed" -ForegroundColor Red
    exit 1
}

# Build optimized version  
Write-Host "Building optimized version..." -ForegroundColor Cyan
go build -ldflags "-s -w -X main.version=$Version -X main.commit=$commit -X main.date=$date" -o release/bhunter-optimized.exe
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Optimized build completed" -ForegroundColor Green
} else {
    Write-Host "✗ Optimized build failed" -ForegroundColor Red
    exit 1
}

# Show build results
Write-Host "`nBuild Summary:" -ForegroundColor Green
Get-ChildItem release/*.exe | ForEach-Object {
    $sizeMB = [math]::Round($_.Length / 1MB, 1)
    Write-Host "  $($_.Name): ${sizeMB} MB" -ForegroundColor White
}

# Test version info
Write-Host "`nTesting release..." -ForegroundColor Cyan
$versionOutput = & ".\release\bhunter.exe" --version
Write-Host $versionOutput -ForegroundColor Yellow

Write-Host "`n🎉 Release build completed successfully!" -ForegroundColor Green
Write-Host "Files available in ./release/ directory" -ForegroundColor White
