[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://127.0.0.1:8080'
)

$ErrorActionPreference = 'Stop'
$base = $BaseUrl.TrimEnd('/')
$username = [Environment]::GetEnvironmentVariable('GO_SERVICE_ADMIN_USERNAME')
$password = [Environment]::GetEnvironmentVariable('GO_SERVICE_ADMIN_PASSWORD')

if ([string]::IsNullOrWhiteSpace($username)) {
    throw 'GO_SERVICE_ADMIN_USERNAME is not set in the current process.'
}
if ([string]::IsNullOrWhiteSpace($password)) {
    throw 'GO_SERVICE_ADMIN_PASSWORD is not set in the current process.'
}

function Invoke-JsonEndpoint {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$Method,
        [hashtable]$Headers = @{},
        [object]$Body
    )

    $request = @{
        Uri         = $Uri
        Method      = $Method
        Headers     = $Headers
        TimeoutSec  = 10
        ErrorAction = 'Stop'
    }
    if ($null -ne $Body) {
        $request.ContentType = 'application/json'
        $request.Body = ($Body | ConvertTo-Json -Compress)
    }

    try {
        $response = Invoke-WebRequest @request
        $content = if ([string]::IsNullOrWhiteSpace($response.Content)) { '{}' } else { $response.Content }
        return [pscustomobject]@{
            StatusCode = [int]$response.StatusCode
            Json       = ($content | ConvertFrom-Json)
        }
    } catch {
        $statusCode = 0
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
            $statusCode = [int]$_.Exception.Response.StatusCode
        }
        throw "HTTP request failed with status ${statusCode}: $Method $Uri"
    }
}

$health = Invoke-JsonEndpoint -Uri "$base/health" -Method 'GET'
if ($health.StatusCode -ne 200) {
    throw "Health check failed with status $($health.StatusCode)."
}

$login = Invoke-JsonEndpoint -Uri "$base/api/v1/admin/auth/login" -Method 'POST' -Body @{
    username = $username
    password = $password
}
if ($login.StatusCode -ne 200 -or $login.Json.code -ne 0 -or [string]::IsNullOrWhiteSpace($login.Json.data.access_token)) {
    throw "Admin login failed with status $($login.StatusCode)."
}

$users = Invoke-JsonEndpoint -Uri "$base/api/v1/admin/users?page=1&page_size=20" -Method 'GET' -Headers @{
    Authorization = "Bearer $($login.Json.data.access_token)"
}
if ($users.StatusCode -ne 200 -or $users.Json.code -ne 0 -or $null -eq $users.Json.data.items -or $null -eq $users.Json.data.stats) {
    throw "Admin user list failed with status $($users.StatusCode)."
}

$items = @($users.Json.data.items)
Write-Output 'ADMIN LIVE SMOKE PASS'
Write-Output "health_status=$($health.StatusCode)"
Write-Output "login_status=$($login.StatusCode)"
Write-Output "users_status=$($users.StatusCode)"
Write-Output "user_count=$($items.Count)"
Write-Output "stats_total=$($users.Json.data.stats.total)"
Write-Output "stats_active=$($users.Json.data.stats.active)"
Write-Output "stats_disabled=$($users.Json.data.stats.disabled)"
