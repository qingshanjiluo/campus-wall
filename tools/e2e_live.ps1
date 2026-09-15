# CampusWall live E2E smoke (against Pages production; through _worker.js proxy -> Worker API)
# Usage: powershell -File tools/e2e_live.ps1 [-BaseUrl https://campus-wall-673.pages.dev]
param(
  [string]$BaseUrl = "https://campus-wall-673.pages.dev",
  [string]$Suffix  = (Get-Random -Maximum 99999)
)
$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$pass = 0; $fail = 0; $failList = @()

function Step($name, [scriptblock]$body) {
  try { & $body; $script:pass++; Write-Host "  PASS $name" -ForegroundColor Green }
  catch { $script:fail++; $script:failList += $name; Write-Host "  FAIL $name :: $($_.Exception.Message)" -ForegroundColor Red }
}
function Assert($cond, $msg) { if (-not $cond) { throw $msg } }
function Api($method, $path, $body = $null, $token = $null) {
  $hdr = @{}
  if ($token) { $hdr["Authorization"] = "Bearer $token" }
  $p = @{ Uri = "$BaseUrl$path"; Method = $method; Headers = $hdr; TimeoutSec = 30 }
  if ($null -ne $body) {
    $json = $body | ConvertTo-Json -Depth 8
    $p.Body = [Text.Encoding]::UTF8.GetBytes($json)
    $p.ContentType = "application/json; charset=utf-8"
  }
  Invoke-RestMethod @p
}

$u = "e2e_$Suffix"
Write-Host "== live E2E @ $BaseUrl (user=$u) =="

Step "register"  { $r = Api POST "/api/auth/register" @{username=$u; email="$u@test.dev"; password="pass1234"}; Assert($r.token) "no token"; $script:tk=$r.token }
Step "login"     { $r = Api POST "/api/auth/login" @{username=$u; password="pass1234"}; Assert($r.token) "no token" }
Step "me"        { $r = Api GET "/api/auth/me" $null $tk; Assert($r.username -eq $u) "wrong user" }
Step "update profile" { $r = Api PUT "/api/auth/me" @{bio="e2e bio"; avatar="/static/images/default-avatar.svg"} $tk; Assert($r) "fail" }
Step "stations list" { $r = Api GET "/api/stations"; Assert($r.Count -ge 10) "stations<10" }
Step "station detail" { $r = Api GET "/api/stations/1"; Assert($r.id -eq 1) "bad detail" }
Step "join station" { $r = Api POST "/api/stations/1/join" @{} $tk; Assert($r) "join fail" }
Step "my stations" { $r = Api GET "/api/stations/mine" $null $tk; Assert($r) "fail" }
Step "create post" {
  $r = Api POST "/api/posts" @{station_id=1; title="e2e post $Suffix"; content="body line1`n![img](/static/uploads/x.png)"} $tk
  $script:pid1 = if ($r.id) {$r.id} else {$r.post.id}; Assert($pid1) "no id" }
Step "post detail" { $r = Api GET "/api/posts/$pid1" $null $tk; Assert($r.title -like "e2e*") "bad detail" }
Step "vote create" {
  $r = Api POST "/api/posts" @{station_id=1; title="e2e vote $Suffix"; content="choose one"; post_type="vote"; vote_options=@("OptA","OptB")} $tk
  $script:pid2 = if ($r.id) {$r.id} else {$r.post.id}; Assert($pid2) "no vote post" }
Step "vote shaping" { $r = Api GET "/api/posts/$pid2" $null $tk; Assert($r.vote_options -like "*OptA*") "no vote_options" }
Step "vote cast"    { $r = Api POST "/api/posts/$pid2/vote" @{option_index=0} $tk; Assert($r) "vote fail" }
Step "vote twice rejected" {
  try { Api POST "/api/posts/$pid2/vote" @{option_index=1} $tk; throw "expected HTTP 400" }
  catch { if ($_.Exception.Response -and $_.Exception.Response.StatusCode.value__ -eq 400) { } else { throw } } }
Step "vote counts"  { $r = Api GET "/api/posts/$pid2" $null $tk; Assert([int]$r.vote_counts."0" -eq 1) ("counts=" + ($r.vote_counts | ConvertTo-Json -Compress)) }
Step "link create"  {
  $r = Api POST "/api/posts" @{station_id=1; title="e2e link"; content="ref"; post_type="link"; link_url="https://example.com"} $tk
  $script:pid3 = if ($r.id) {$r.id} else {$r.post.id}; Assert($pid3) "no link post" }
Step "link shaping" { $r = Api GET "/api/posts/$pid3"; Assert($r.link_url -eq "https://example.com") "link_url=$($r.link_url)" }
Step "like toggle"  { $r = Api POST "/api/posts/$pid1/like" @{} $tk; Assert($r -ne $null) "fail" }
Step "comment"      { $r = Api POST "/api/posts/$pid1/comments" @{content="e2e comment"} $tk; Assert($r) "fail" }
Step "comments list"{ $r = Api GET "/api/posts/$pid1/comments"; Assert($r -ne $null) "fail" }
Step "checkin"      { $r = Api POST "/api/checkin" @{} $tk; Assert($r) "fail" }
Step "checkin status" { $r = Api GET "/api/checkin/status" $null $tk; Assert($r) "fail" }
Step "shop items"   { $r = Api GET "/api/shop/items"; Assert($r.Count -ge 1) "no items" }
Step "growth"       { $r = Api GET "/api/growth" $null $tk; Assert($r) "fail" }
Step "romance save" { $r = Api POST "/api/romance/profile" @{nickname="e2e"; gender="u"; age=20; department="CS"; hobbies=@("code","draw")} $tk; Assert($r) "save1 fail" }
Step "romance update-path" { $r = Api POST "/api/romance/profile" @{nickname="e2e2"; gender="u"; hobbies=@("a","b","c")} $tk; Assert($r) "UPDATE list-hobbies regression" }
Step "gossip create"{ $r = Api POST "/api/gossip" @{content="e2e gossip"} $tk; Assert($r) "fail" }
Step "trades list"  { $r = Api GET "/api/trades"; Assert($r -ne $null) "fail" }
Step "trade create" { $r = Api POST "/api/trade" @{title="e2e item"; content="good cond"; price=9.9; condition="like_new"} $tk; Assert($r.post_id -or $r.trade_id) "fail" }
Step "search"       { $r = Api GET "/api/search?q=e2e"; Assert($r) "fail" }
Step "recommend posts" { $r = Api GET "/api/recommend/posts"; Assert($r -ne $null) "fail" }
Step "admin login"  { $r = Api POST "/api/auth/login" @{username="admin"; password="admin123"}; Assert($r.token) "no admin token"; $script:atk=$r.token }
Step "admin stats today fields" { $r = Api GET "/api/admin/stats" $null $atk; Assert($r.users -ge 1) "no users count"; Assert($r.PSObject.Properties.Name -contains "today_posts") "missing today_posts" }
Step "admin users"  { $r = Api GET "/api/admin/users?limit=5" $null $atk; Assert($r) "fail" }
Step "notifications"{ $r = Api GET "/api/notifications" $null $tk; Assert($r -ne $null) "fail" }
Step "upload post image" {
  Assert($tk) "no auth token (earlier steps failed)"
  $png = [Convert]::FromBase64String("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
  Add-Type -AssemblyName System.Net.Http
  $client = New-Object Net.Http.HttpClient
  $client.Timeout = [TimeSpan]::FromSeconds(40)
  $client.DefaultRequestHeaders.Add("Authorization", "Bearer $tk")
  $content = New-Object Net.Http.MultipartFormDataContent
  $cc = New-Object Net.Http.ByteArrayContent(,$png); $cc.Headers.ContentType = [Net.Http.Headers.MediaTypeHeaderValue]::Parse("image/png")
  $content.Add($cc, "file", "e2e.png")
  $resp = $client.PostAsync("$BaseUrl/api/posts/upload-image", $content).Result
  Assert($resp.StatusCode -eq 200) "http=$($resp.StatusCode)"
  $txt = $resp.Content.ReadAsStringAsync().Result | ConvertFrom-Json
  Assert($txt.url) "no url in resp"
  $script:imgurl = $txt.url }
Step "serve uploaded image" {
  Assert($imgurl) "no image url from upload"
  $resp = Invoke-WebRequest -Uri "$BaseUrl$imgurl" -TimeoutSec 30 -Method GET -UseBasicParsing
  Assert($resp.StatusCode -eq 200) "img http=$($resp.StatusCode)"
  Assert($resp.RawContentLength -gt 0) "empty image body" }

Write-Host ""
Write-Host ("== live E2E: pass={0} fail={1} ==" -f $pass, $fail) -ForegroundColor $(if($fail){'Red'}else{'Green'})
if ($fail) { $failList | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }; exit 1 } else { exit 0 }
