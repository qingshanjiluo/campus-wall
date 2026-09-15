# CampusWall Worker API smoke test — 按真实路由表对齐（本地 wrangler dev @ 8787）
$ErrorActionPreference = 'Continue'
$Base = 'http://127.0.0.1:8787'
$Results = @()

function Invoke-Api {
    param($Name, $Method, $Path, $Body = $null, $Token = $null, $ExpectStatus = $null)
    Start-Sleep -Milliseconds 250
    $headers = @{}
    if ($Token) { $headers['Authorization'] = "Bearer $Token" }
    $uri = "$Base$Path"
    try {
        if ($null -ne $Body) {
            $json = $Body | ConvertTo-Json -Depth 10
            $resp = Invoke-WebRequest -Uri $uri -Method $Method -Headers $headers -ContentType 'application/json; charset=utf-8' -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -UseBasicParsing -TimeoutSec 90
        } else {
            $resp = Invoke-WebRequest -Uri $uri -Method $Method -Headers $headers -UseBasicParsing -TimeoutSec 90
        }
        $status = $resp.StatusCode
        $content = $resp.Content
    } catch {
        $status = $_.Exception.Response.StatusCode.value__
        if (-not $status) { $status = "ERR" }
        try { $content = $_.ErrorDetails.Message } catch { $content = $_.Exception.Message }
    }
    $ok = if ($ExpectStatus) { $status -eq $ExpectStatus } else { $status -ge 200 -and $status -lt 300 }
    $sample = "$content" -replace '\s+', ' '
    $script:Results += [pscustomobject]@{ Name = $Name; Status = $status; Pass = $ok; Sample = $sample.Substring(0, [Math]::Min(110, $sample.Length)) }
    if ($ok) { try { return ($content | ConvertFrom-Json) } catch { return $null } } else { return $null }
}

Write-Host "== public lists =="
$null = Invoke-Api "GET /api/stations" GET '/api/stations?limit=3' -ExpectStatus 200
$null = Invoke-Api "GET /api/posts" GET '/api/posts?limit=3' -ExpectStatus 200
$null = Invoke-Api "GET /api/stations/search" GET '/api/stations/search?q=%E8%A1%8C' -ExpectStatus 200
$null = Invoke-Api "GET /api/recommend/search" GET '/api/recommend/search?q=wall' -ExpectStatus 200
$null = Invoke-Api "GET /api/announcements" GET '/api/announcements' -ExpectStatus 200
$null = Invoke-Api "GET /api/kanban/message" GET '/api/kanban/message' -ExpectStatus 200
$null = Invoke-Api "GET /api/stations/categories" GET '/api/stations/categories' -ExpectStatus 200

Write-Host "== auth =="
$u = "smoke" + (Get-Random -Maximum 99999)
$reg = Invoke-Api "POST /api/auth/register" POST '/api/auth/register' @{ username = $u; email = "$u@test.com"; password = "pass123456" } -ExpectStatus 201
$login = Invoke-Api "POST /api/auth/login" POST '/api/auth/login' @{ username = $u; password = "pass123456" } -ExpectStatus 200
$token = if ($login.token) { $login.token } else { $reg.token }
$null = Invoke-Api "GET /api/auth/me" GET '/api/auth/me' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET /api/auth/me anon->401" GET '/api/auth/me' -ExpectStatus 401
$adm = Invoke-Api "POST login admin" POST '/api/auth/login' @{ username = 'admin'; password = 'admin123' } -ExpectStatus 200
$atoken = $adm.token

Write-Host "== station flow =="
$st = Invoke-Api "POST /api/stations" POST '/api/stations' @{ name = "smoke station $u"; description = 'smoke desc'; tags = @('test'); is_public = $true } -Token $token -ExpectStatus 201
$sid = if ($st) { $st.id } else { $null }
if (-not $sid -and $st.station) { $sid = $st.station.id }
if ($sid) {
    $null = Invoke-Api "GET station detail" GET "/api/stations/$sid" -ExpectStatus 200
    $null = Invoke-Api "GET station posts" GET "/api/stations/$sid/posts" -ExpectStatus 200
    $null = Invoke-Api "GET station members" GET "/api/stations/$sid/members" -ExpectStatus 200
    $null = Invoke-Api "PUT station update" PUT "/api/stations/$sid" @{ description = 'updated desc' } -Token $token -ExpectStatus 200
    $null = Invoke-Api "GET stations/mine" GET '/api/stations/mine' -Token $token -ExpectStatus 200
}

Write-Host "== post flow =="
if ($sid) {
    $po = Invoke-Api "POST /api/posts" POST '/api/posts' @{ station_id = $sid; title = 'smoke post'; content = 'body text'; post_type = 'text' } -Token $token
    $pid2 = if ($po) { $po.id } else { $null }
    if (-not $pid2 -and $po.post) { $pid2 = $po.post.id }
    if ($pid2) {
        $null = Invoke-Api "GET post detail" GET "/api/posts/$pid2" -ExpectStatus 200
        $null = Invoke-Api "POST like" POST "/api/posts/$pid2/like" @{} -Token $token -ExpectStatus 200
        $cm = Invoke-Api "POST comment" POST "/api/posts/$pid2/comments" @{ content = 'comment test' } -Token $token
        $null = Invoke-Api "PUT edit post" PUT "/api/posts/$pid2" @{ title = 'smoke post edited' } -Token $token -ExpectStatus 200
        $null = Invoke-Api "GET versions" GET "/api/posts/$pid2/versions" -Token $token -ExpectStatus 200
        $null = Invoke-Api "POST favorite" POST "/api/favorites/post/$pid2" @{} -Token $token
        $null = Invoke-Api "GET favorite status" GET "/api/favorites/status/post/$pid2" -Token $token -ExpectStatus 200
        $null = Invoke-Api "POST report" POST '/api/reports' @{ target_type = 'post'; target_id = $pid2; reason = 'smoke report' } -Token $token
        if ($cm -and $cm.id) { $null = Invoke-Api "POST comment like" POST "/api/social/comment/$($cm.id)/like" @{} -Token $token -ExpectStatus 200 }
    }
}

Write-Host "== extended modules =="
$null = Invoke-Api "GET checkin status" GET '/api/checkin/status' -Token $token -ExpectStatus 200
$null = Invoke-Api "POST checkin" POST '/api/checkin' @{} -Token $token -ExpectStatus 200
$null = Invoke-Api "GET shop items" GET '/api/shop/items' -ExpectStatus 200
$null = Invoke-Api "GET shop coins" GET '/api/shop/coins' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET shop orders" GET '/api/shop/orders' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET transactions" GET '/api/shop/transactions' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET gossip" GET '/api/gossip' -ExpectStatus 200
$null = Invoke-Api "POST gossip" POST '/api/gossip' @{ content = 'anonymous tree hole smoke' } -Token $token
$null = Invoke-Api "GET trade" GET '/api/trade' -ExpectStatus 200
$null = Invoke-Api "POST trade" POST '/api/trade' @{ title = 'smoke trade'; content = 'desc'; price = 9.9; category = 'books'; contact = 'qq123'; original_price = 20; condition = 'good' } -Token $token
$null = Invoke-Api "GET romance profiles" GET '/api/romance/profiles' -ExpectStatus 200
$null = Invoke-Api "GET romance tasks" GET '/api/romance/tasks' -ExpectStatus 200
$null = Invoke-Api "GET recommend posts" GET '/api/recommend/posts' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET recommend interests" GET '/api/recommend/interests' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET social notifications" GET '/api/social/notifications' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET unread count" GET '/api/social/notifications/unread-count' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET identity groups" GET '/api/identity/groups' -ExpectStatus 200
$null = Invoke-Api "GET identity my" GET '/api/identity/my' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET posts liked" GET '/api/posts/liked' -Token $token -ExpectStatus 200
$null = Invoke-Api "GET favorites" GET '/api/favorites' -Token $token -ExpectStatus 200

Write-Host "== admin modules =="
$null = Invoke-Api "GET admin stats" GET '/api/admin/stats' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin stats series" GET '/api/admin/stats/series' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin users" GET '/api/admin/users' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin posts" GET '/api/admin/posts' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin stations" GET '/api/admin/stations' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin logs" GET '/api/admin/logs' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin announcements" GET '/api/admin/announcements' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin identity-groups" GET '/api/admin/identity-groups' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET reports (admin)" GET '/api/reports' -Token $atoken -ExpectStatus 200
$null = Invoke-Api "GET admin stats as user->403" GET '/api/admin/stats' -Token $token -ExpectStatus 403

Write-Host ""
Write-Host "================ SUMMARY ================"
$Results | Format-Table -AutoSize -Wrap Name, Status, Pass
$fail = $Results | Where-Object { -not $_.Pass }
Write-Host ("total={0} pass={1} fail={2}" -f $Results.Count, ($Results.Count - $fail.Count), $fail.Count)
if ($fail) { $fail | ForEach-Object { Write-Host ("FAIL: {0} [{1}] {2}" -f $_.Name, $_.Status, $_.Sample) } }
