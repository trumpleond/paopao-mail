# Smoke test against a running server at http://127.0.0.1:8080
$ErrorActionPreference = "Stop"
$Base = if ($env:BASE_URL) { $env:BASE_URL } else { "http://127.0.0.1:8080" }
$Headers = @{}
if ($env:API_KEY) { $Headers["X-API-Key"] = $env:API_KEY }

Write-Host "== health =="
Invoke-RestMethod "$Base/health" | ConvertTo-Json -Compress

Write-Host "== import =="
$importBody = @"
smoke1@example.com----pass1
smoke2@example.com----pass2
"@
$imp = Invoke-RestMethod -Method Post -Uri "$Base/api/accounts/import" -Headers $Headers -ContentType "text/plain; charset=utf-8" -Body $importBody
$imp | ConvertTo-Json -Compress

Write-Host "== pick xai =="
$pick = Invoke-RestMethod -Method Post -Uri "$Base/api/accounts/pick" -Headers $Headers -ContentType "application/json" -Body '{"platform":"xai"}'
$pick | ConvertTo-Json -Compress
$id = $pick.data.id

Write-Host "== mark =="
Invoke-RestMethod -Method Post -Uri "$Base/api/accounts/$id/mark" -Headers $Headers -ContentType "application/json" -Body '{"platform":"xai"}' | ConvertTo-Json -Compress

Write-Host "== stats =="
Invoke-RestMethod "$Base/api/stats" -Headers $Headers | ConvertTo-Json -Depth 5 -Compress

Write-Host "OK"
