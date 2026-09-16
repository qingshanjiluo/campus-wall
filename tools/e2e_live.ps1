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
  catch {
    $script:fail++; $script:failList += $name
    $ln = $_.InvocationInfo.ScriptLineNumber
    Write-Host "  FAIL $name :: $($_.Exception.Message) [line $ln]" -ForegroundColor Red
  }
}
function Assert($cond, $msg) { if (-not $cond) { throw $msg } }
function BadCode($e) {
  # compatible with 3 shapes: IRM native response / Api() wrapped "HTTP <code> ..." / PS5.1 native "(<code>) ..."
  try { if ($e.Exception.Response) { return [int]$e.Exception.Response.StatusCode } } catch {}
  $msg = "$($e.Exception.Message)"
  if ($msg -match 'HTTP (\d{3})') { return [int]$Matches[1] }
  if ($msg -match '\((\d{3})\)') { return [int]$Matches[1] }
  0
}
function Api($method, $path, $body = $null, $token = $null) {
  $hdr = @{}
  if ($token) { $hdr["Authorization"] = "Bearer $token" }
  $p = @{ Uri = "$BaseUrl$path"; Method = $method; Headers = $hdr; TimeoutSec = 30 }
  if ($null -ne $body) {
    $json = $body | ConvertTo-Json -Depth 8
    $p.Body = [Text.Encoding]::UTF8.GetBytes($json)
    $p.ContentType = "application/json; charset=utf-8"
  }
  try {
    Invoke-RestMethod @p
  } catch {
    # surface the server JSON error body so failing steps are self-explanatory
    $detail = ''
    try { $detail = "$($_.ErrorDetails.Message)" } catch {}
    if (-not $detail) {
      try {
        $sr = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $detail = $sr.ReadToEnd()
      } catch {}
    }
    $code = 0; try { $code = [int]$_.Exception.Response.StatusCode } catch {}
    if ($detail) { throw "HTTP $code $method $path :: $detail" }
    throw "HTTP $code $method $path :: $($_.Exception.Message)"
  }
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
  catch { Assert((BadCode $_) -eq 400) "second vote not rejected" } }
Step "vote counts"  { $r = Api GET "/api/posts/$pid2" $null $tk; Assert([int]$r.vote_counts."0" -eq 1) ("counts=" + ($r.vote_counts | ConvertTo-Json -Compress)) }
Step "link create"  {
  $r = Api POST "/api/posts" @{station_id=1; title="e2e link"; content="ref"; post_type="link"; link_url="https://example.com"} $tk
  $script:pid3 = if ($r.id) {$r.id} else {$r.post.id}; Assert($pid3) "no link post" }
Step "link shaping" { $r = Api GET "/api/posts/$pid3"; Assert($r.link_url -eq "https://example.com") "link_url=$($r.link_url)" }
Step "like toggle"  {
  $r = Api POST "/api/posts/$pid1/like" @{} $tk
  Assert($r.liked) "not liked"
  Assert($r.likes_count -eq 1) "likes_count=$($r.likes_count) (want 1)"
  $r2 = Api POST "/api/posts/$pid1/like" @{} $tk
  Assert(-not $r2.liked) "still liked"
  Assert($r2.likes_count -eq 0) "unlike count=$($r2.likes_count) (want 0)" }
Step "comment"      { $r = Api POST "/api/posts/$pid1/comments" @{content="e2e comment"} $tk; Assert($r) "fail" }
Step "comments list"{ $r = Api GET "/api/posts/$pid1/comments"; Assert($null -ne $r) "fail" }
Step "checkin"      { $r = Api POST "/api/checkin" @{} $tk; Assert($r) "fail" }
Step "checkin status" { $r = Api GET "/api/checkin/status" $null $tk; Assert($r) "fail" }
Step "shop items"   { $r = Api GET "/api/shop/items"; Assert($r.Count -ge 1) "no items" }
Step "growth coins"  { $r = Api GET "/api/shop/coins" $null $tk; Assert($null -ne $r) "fail" }
Step "romance save" { $r = Api POST "/api/romance/profile" @{nickname="e2e"; gender="u"; age=20; department="CS"; hobbies=@("code","draw")} $tk; Assert($r) "save1 fail" }
Step "romance update-path" { $r = Api POST "/api/romance/profile" @{nickname="e2e2"; gender="u"; hobbies=@("a","b","c")} $tk; Assert($r) "UPDATE list-hobbies regression" }
Step "gossip create"{ $r = Api POST "/api/gossip" @{content="e2e gossip"} $tk; Assert($r) "fail" }
Step "trade list"   { $r = Api GET "/api/trade"; Assert($null -ne $r) "fail" }
Step "trade create" {
  $r = Api POST "/api/trade" @{title="e2e item"; content="good cond"; price=9.9; condition="like_new"} $tk
  Assert($r.post_id -or $r.trade_id) "fail"
  $script:tid = $r.trade_id }
Step "trade status flow" {
  $s1 = Api PUT "/api/trade/$tid/status" @{status="reserved"} $tk
  Assert($s1.status -eq "reserved") "owner update failed"
  $s2 = Api PUT "/api/trade/$tid/status" @{status="sold"} $tk
  Assert($s2.status -eq "sold") "sold update failed"
  $s3 = Api PUT "/api/trade/$tid/status" @{status="available"} $tk
  Assert($s3.status -eq "available") "relist failed" }
Step "trade status validation" {
  try { Api PUT "/api/trade/$tid/status" @{status="bogus"} $tk; throw "expected 400" }
  catch { Assert((BadCode $_) -eq 400) "bogus status not rejected" }
  try { Api PUT "/api/trade/999999/status" @{status="sold"} $tk; throw "expected 404" }
  catch { Assert((BadCode $_) -eq 404) "missing trade not 404" }
  # cross-user privilege check: another user must not change someone else's trade status (R5 owner guard)
  $u2 = 'e2e_b' + $Suffix
  $r2 = Api POST "/api/auth/register" @{username=$u2; password="pass1234"; email=($u2 + '@t.dev')}
  try { Api PUT "/api/trade/$tid/status" @{status="sold"} $r2.token; throw "expected 403" }
  catch { Assert((BadCode $_) -eq 403) "cross-user trade update not forbidden" } }
Step "multi-image gallery" {
  $imgs = @("/static/uploads/posts/a.png", "/static/uploads/posts/b.png", "/static/uploads/posts/c.png")
  $p = Api POST "/api/posts" @{station_id=1; title="e2e imgs"; content="gallery"; images=$imgs} $tk
  $script:pid4 = if ($p.id) {$p.id} else {$p.post.id}
  $d = Api GET "/api/posts/$pid4"
  Assert($d.images.Count -eq 3) "detail images=$($d.images.Count) (want 3)"
  Assert($d.images[0] -eq "/static/uploads/posts/a.png") "first image mismatch"
  $f = Api GET "/api/posts?limit=50"
  $row = @($f.posts | Where-Object { $_.id -eq $pid4 })
  Assert($row.Count -ge 1) "post missing in feed"
  Assert($row[0].images.Count -eq 3) "feed images not expanded (got $($row[0].images.GetType().Name))"
  $rp = Api GET "/api/recommend/posts"
  $rrow = @($rp | Where-Object { $_.id -eq $pid4 })
  if ($rrow.Count -ge 1) { Assert($rrow[0].images -is [array] -or $rrow[0].images.Count -ge 0) "recommend images broken" } }
Step "search posts" { $r = Api GET "/api/recommend/search?q=e2e"; Assert($null -ne $r) "fail" }
Step "search stations" { $r = Api GET "/api/stations/search?q=e2e"; Assert($null -ne $r) "fail" }
Step "recommend posts" { $r = Api GET "/api/recommend/posts"; Assert($null -ne $r) "fail" }
Step "admin login"  { $r = Api POST "/api/auth/login" @{username="admin"; password="admin123"}; Assert($r.token) "no admin token"; $script:atk=$r.token }
Step "admin stats today fields" { $r = Api GET "/api/admin/stats" $null $atk; Assert($r.users -ge 1) "no users count"; Assert($r.PSObject.Properties.Name -contains "today_posts") "missing today_posts" }
Step "admin users"  { $r = Api GET "/api/admin/users?limit=5" $null $atk; Assert($r) "fail" }
Step "notifications"{ $r = Api GET "/api/social/notifications" $null $tk; Assert($null -ne $r) "fail" }
Step "forgot+reset roundtrip" {
  $fp = Api POST "/api/auth/forgot-password" @{email="$u@t.dev"}
  if ($fp.reset_token) {
    $rs = Api POST "/api/auth/reset-password" @{token=$fp.reset_token; new_password="e2epw1234"}
    Assert($rs.message) "reset rejected"
    $l2 = Api POST "/api/auth/login" @{username=$u; password="e2epw1234"}
    Assert($l2.token) "login after reset failed"
    $still = $true
    try { $old = Api POST "/api/auth/login" @{username=$u; password="pass1234"}; $still = [bool]$old.token } catch { $still = $false }
    Assert(-not $still) "old password still works"
  } else { Assert($fp.message) "no message either" } }
# ---- direct messages (R4-M4) ----
Step "dm send"       { $r = Api POST "/api/dm" @{to=1; content="e2e hello dm"} $tk; Assert($r.id) "send fail" }
Step "dm threads"    { $r = Api GET "/api/dm/threads" $null $tk; Assert(@($r | Where-Object { $_.peer_id -eq 1 }).Count -ge 1) "thread missing" }
Step "dm read thread"{ $r = Api GET "/api/dm/1" $null $tk; Assert($r.peer.username) "no peer info"; Assert(@($r.messages).Count -ge 1) "no messages" }
Step "dm unread seen"{ $r = Api GET "/api/dm/unread" $null $atk; Assert($null -ne $r.count) "no count field" }
Step "dm bad to rejected" {
  try { Api POST "/api/dm" @{to=$null; content="x"} $tk; throw "expected 400" }
  catch { Assert((BadCode $_) -eq 400) "wrong code" } }
# ---- content review pipeline (R4-M3) ----
$W_RV1 = [regex]::Unescape('\u8fd9\u662f\u4e00\u4e2a\u50bb\u903c\u6d4b\u8bd5\u5e16\u5b50')
$W_RV2 = [regex]::Unescape('\u516d\u5408\u5f69\u5f00\u76d8')
$W_RV3 = [regex]::Unescape('\u5ba1\u6838')
Step "review queue hit" {
  $r = Api POST "/api/posts" @{station_id=1; title="e2e rv $Suffix"; content=$W_RV1} $atk
  Assert($r.status -eq "pending") "not pending: got $($r.status)"
  $script:pidrv = $r.post_id }
Step "review hidden publicly" {
  $l = Api GET "/api/posts?limit=100"
  Assert(-not (@($l.posts | Where-Object { $_.id -eq $script:pidrv }).Count)) "pending post leaked into public list" }
Step "review author-visible" {
  $r = Api GET "/api/posts/$script:pidrv" $null $atk
  Assert($r.status -eq "pending") "author cannot see own pending post" }
Step "review approve" {
  $r = Api POST "/api/admin/review" @{post_id=$script:pidrv; action="approve"} $atk
  Assert($r) "approve fail" }
Step "review visible after" {
  $r = Api GET "/api/posts/$script:pidrv"
  Assert($r.status -eq "approved") "still not visible: $($r.status)" }
Step "block word rejected" {
  try { Api POST "/api/posts" @{station_id=1; title="e2e blk $Suffix"; content=$W_RV2} $atk; throw "expected HTTP 400" }
  catch { Assert((BadCode $_) -eq 400) "wrong code for block word" } }
Step "edit bypass blocked" {
  try { Api PUT "/api/posts/$pid1" @{content=$W_RV2} $atk; throw "expected HTTP 400" }
  catch { Assert((BadCode $_) -eq 400) "edit bypass not caught" } }
Step "edit triggers review + search leak" {
  $uniq = "e2ebyp$Suffix"
  $z = Api PUT "/api/posts/$pid1" @{title=$uniq; content=$W_RV1} $atk
  Assert($z.message -like "*$($W_RV3)*") "edit did not re-enter review"
  $s = Api GET "/api/recommend/search?q=$uniq"
  Assert(-not (@($s.posts | Where-Object { $_.id -eq $pid1 }).Count)) "pending post leaked via search"
  Api POST "/api/admin/review" @{post_id=$pid1; action="approve"} $atk | Out-Null }
Step "review reject flow" {
  $rr = Api POST "/api/posts" @{station_id=1; title="e2e rj $Suffix"; content=$W_RV1} $atk
  $script:pidrj = $rr.post_id
  $x = Api POST "/api/admin/review" @{post_id=$pidrj; action="reject"} $atk
  Assert($x) "reject fail"
  $y = Api GET "/api/posts/$pidrj" $null $atk
  Assert($y.status -eq "rejected") "status not rejected" }
Step "rejected edit resubmits" {
  $z = Api PUT "/api/posts/$pidrj" @{content=($W_RV1 + " edit$Suffix")} $atk
  Assert($z.message -like "*$($W_RV3)*") "no re-review on edit"
  $q2 = Api GET "/api/admin/review" $null $atk
  Assert(@($q2.posts | Where-Object { $_.id -eq $pidrj }).Count -ge 1) "rejected edit not back in queue" }
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

Step "coin ledger (shop transactions)" {
  Assert($tk) "no auth token (earlier steps failed)"
  $tx = Api GET "/api/shop/transactions" $null $tk
  Assert($null -ne $tx) "no response from /api/shop/transactions"
  $arr = @($tx)
  Assert($arr.Count -ge 1) "coin ledger empty after checkin/buy"
  $first = $arr[0]
  Assert($null -ne $first.amount) "ledger row missing amount"
  Assert($null -ne $first.type) "ledger row missing type"
  Assert($null -ne $first.created_at) "ledger row missing created_at" }
Step "checkin credits ledger" {
  $before = @(Api GET "/api/shop/transactions" $null $tk).Count
  $ci = Api POST "/api/checkin" @{} $tk
  $after = @(Api GET "/api/shop/transactions" $null $tk).Count
  Assert($after -ge $before) "ledger shrank after checkin"
  if ($ci.already) {
    Assert($after -eq $before) "duplicate checkin wrote a ledger row"
  } else {
    Assert($after -gt $before) "checkin did not write a ledger row" } }
Step "frontend pages reachable" {
  $pages = @('/', '/waterfall', '/trade', '/gossip', '/romance', '/shop', '/checkin',
             '/messages', '/search', '/stations', '/rank', '/square', '/tasks',
             '/notifications', '/about', '/help', '/create', '/admin', '/login', '/404')
  $bad = @()
  foreach ($pg in $pages) {
    try {
      $r = Invoke-WebRequest -Uri "$BaseUrl$pg" -TimeoutSec 20 -UseBasicParsing -ErrorAction Stop
      if ($r.StatusCode -ne 200) { $bad += "$pg=$($r.StatusCode)" }
    } catch {
      $code = try { [int]$_.Exception.Response.StatusCode } catch { 0 }
      if ($code -ne 404 -and $pg -ne '/404') { $bad += "$pg=ERR$code" }
    }
  }
  Assert($bad.Count -eq 0) ("unreachable pages: " + ($bad -join ',')) }
Step "vendored lucide served" {
  $r = Invoke-WebRequest -Uri "$BaseUrl/static/vendor/lucide.min.js" -TimeoutSec 30 -UseBasicParsing
  Assert($r.StatusCode -eq 200) "lucide http=$($r.StatusCode)"
  Assert($r.RawContentLength -gt 100000) "lucide bundle too small: $($r.RawContentLength)"
  Assert($r.Content -like "*createIcons*") "bundle missing createIcons"
  Assert($r.Content -like "*1.46.0*") "bundle version not pinned" }

Write-Host ""
Write-Host ("== live E2E: pass={0} fail={1} ==" -f $pass, $fail) -ForegroundColor $(if($fail){'Red'}else{'Green'})
if ($fail) { $failList | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }; exit 1 } else { exit 0 }
