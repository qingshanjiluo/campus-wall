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
# curl honors NO_PROXY (localhost bypass); Invoke-WebRequest on PS5.1 does a ~20s
# WPAD auto-detect even for localhost. Reachability/status checks use curl.
# Cross-platform: `curl` is an IWR alias on PS5.1/Windows (use curl.exe there), and
# the null device differs (NUL vs /dev/null).
$script:NullDev = if ($env:OS -eq 'Windows_NT') { 'NUL' } else { '/dev/null' }
function Invoke-CurlRaw($curlArgs) {
  if ($env:OS -eq 'Windows_NT') { & curl.exe @curlArgs 2>&1 } else { & curl @curlArgs 2>&1 }
}
function HttpCode($url) {
  Invoke-CurlRaw @('-s', '-o', $script:NullDev, '-w', '%{http_code}', '--max-time', '25', $url)
}
function HttpCodeSize($url, $token = $null) {
  $a = @('-s', '-o', $script:NullDev, '-w', '%{http_code} %{size_download}', '--max-time', '25', $url)
  if ($token) { $a += @('-H', "Authorization: Bearer $token") }
  Invoke-CurlRaw $a
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
  $out = HttpCodeSize "$BaseUrl$imgurl"
  $parts = "$out".Trim() -split '\s+'
  Assert($parts[0] -eq '200') "img http=$($parts[0])"
  Assert([int]$parts[1] -gt 0) "empty image body" }

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
  # Strict assert: every real public route must return 200; /404 must return 404.
  # (older version swallowed 404s via catch - this now catches page regressions)
  $ok = @('/', '/waterfall', '/world', '/forum', '/expose', '/trade', '/romance',
          '/gossip', '/shop', '/checkin', '/search', '/favorites', '/messages',
          '/notifications', '/create', '/create-station', '/about', '/terms',
          '/privacy', '/reset-password', '/admin', '/profile/1')
  $bad = @()
  foreach ($pg in $ok) {
    $code = HttpCode "$BaseUrl$pg"
    if ("$code" -ne '200') { $bad += "$pg=$code" }
  }
  $code404 = HttpCode "$BaseUrl/404"
  if ("$code404" -ne '404') { $bad += "/404=$code404" }
  Assert($bad.Count -eq 0) ("page status wrong: " + ($bad -join ',')) }
Step "world graph api" {
  $g = Api GET "/api/world/graph"
  Assert($null -ne $g) "no graph"
  Assert($null -ne $g.nodes) "graph missing nodes"
  Assert($g.nodes.Count -ge 3) "graph too small"
  Assert($null -ne $g.relations) "graph missing relations" }
Step "world node create/delete" {
  Assert($tk) "no token (earlier steps failed)"
  $n1 = Api POST "/api/world/nodes" @{name="e2e_w$Suffix"; portrait="E"; tagline="t"} $tk
  Assert($n1.id) "no node id"
  # self-relation (node -> itself) must be rejected with 400
  $selfRejected = $false
  try { Api POST "/api/world/relations" @{from_id=$n1.id; to_id=$n1.id; label="self"} $tk; } catch { $selfRejected = ((BadCode $_) -eq 400) }
  Assert($selfRejected) "self-relation not rejected"
  $d = Api DELETE "/api/world/nodes/$($n1.id)" $null $tk
  Assert($d.message) "node delete failed"
  $after = Api GET "/api/world/graph"
  $gone = -not (@($after.nodes | Where-Object { $_.id -eq $n1.id }).Count)
  Assert($gone) "node not actually removed" }
Step "world relation flow" {
  Assert($tk) "no token (earlier steps failed)"
  $a = Api POST "/api/world/nodes" @{name="e2e_ra$Suffix"} $tk
  $b = Api POST "/api/world/nodes" @{name="e2e_rb$Suffix"} $tk
  Assert($a.id -and $b.id) "nodes not created"
  $rel = Api POST "/api/world/relations" @{from_id=$a.id; to_id=$b.id; label="e2e-rel"; reciprocal=1} $tk
  Assert($rel.id) "relation not created"
  $g = Api GET "/api/world/graph"
  Assert(@($g.relations | Where-Object { $_.id -eq $rel.id }).Count -eq 1) "relation missing from graph"
  $dr = Api DELETE "/api/world/relations/$($rel.id)" $null $tk
  Assert($dr.message) "relation delete failed"
  $g2 = Api GET "/api/world/graph"
  Assert(-not (@($g2.relations | Where-Object { $_.id -eq $rel.id }).Count)) "relation not removed"
  Api DELETE "/api/world/nodes/$($a.id)" $null $tk | Out-Null
  Api DELETE "/api/world/nodes/$($b.id)" $null $tk | Out-Null }
Step "topics trending + filter" {
  Assert($tk) "no token (earlier steps failed)"
  $topic = 'e2etp' + $Suffix
  $tp = Api POST "/api/posts" @{station_id=1; title="e2e tp $Suffix"; content="body"; topics=@($topic, "#$topic", '', ('x'*30))} $tk
  $tpid = $tp.post_id; if (-not $tpid) { $tpid = $tp.post.id }
  Assert($tpid) "topic post create failed"
  # normalize: dedup #form, drop empty, drop over-length -> exactly 1 valid topic
  $det = Api GET "/api/posts/$tpid"
  Assert(@($det.topics).Count -eq 1 -and $det.topics[0] -eq $topic) ("topics not normalized: " + ($det.topics -join '|'))
  # trending list includes the topic
  $tr = Api GET "/api/topics/trending?limit=50"
  Assert((@($tr | Where-Object { $_.name -eq $topic }).Count) -eq 1) "topic missing from trending"
  # topic filter returns the post
  $tf = Api GET ("/api/topics/" + [uri]::EscapeDataString($topic) + "/posts?limit=10")
  Assert((@($tf.posts | Where-Object { $_.id -eq $tpid }).Count) -eq 1) "topic filter did not return the post"
  # post_type alias equals type (expose filtering depends on it)
  $byAlias = Api GET "/api/posts?post_type=vote&limit=50"
  $byType  = Api GET "/api/posts?type=vote&limit=50"
  Assert(@($byAlias.posts).Count -eq @($byType.posts).Count) "post_type alias != type"
  # editing with only topics must succeed (was wrongly rejected as nothing-to-update)
  $t2 = 'e2etp2' + $Suffix
  Api PUT "/api/posts/$tpid" @{topics=@($t2)} $tk | Out-Null
  $det2 = Api GET "/api/posts/$tpid"
  Assert(@($det2.topics).Count -eq 1 -and $det2.topics[0] -eq $t2) "topics edit-replace failed"
  Api DELETE "/api/posts/$tpid" $null $tk | Out-Null
  # after post delete, topic filter must be empty
  $tf2 = Api GET ("/api/topics/" + [uri]::EscapeDataString($t2) + "/posts?limit=10")
  Assert(@($tf2.posts).Count -eq 0) "topics not cleaned on post delete" }
Step "tasks bounty + review payout" {
  Assert($tk -and $atk) "tokens missing (earlier steps failed)"
  # bounty: publisher (admin) prepays points, user completes, publisher approves
  $pts0 = (Api GET "/api/shop/coins" $null $atk).points
  $bt = Api POST "/api/tasks" @{kind="bounty"; title="e2e bounty $Suffix"; description="d"; reward_points=10} $atk
  Assert($bt.id) "bounty create failed"
  $pts1 = (Api GET "/api/shop/coins" $null $atk).points
  Assert($pts0 - $pts1 -eq 10) "bounty prepay did not deduct 10 points"
  Api POST "/api/tasks/$($bt.id)/claim" @{} $tk | Out-Null
  $dupRejected = $false
  try { Api POST "/api/tasks/$($bt.id)/claim" @{} $tk } catch { $dupRejected = ((BadCode $_) -eq 400) }
  Assert($dupRejected) "duplicate claim not rejected"
  $up0 = (Api GET "/api/shop/coins" $null $tk).points
  $sub = Api POST "/api/tasks/$($bt.id)/complete" @{proof="e2e done"} $tk
  Assert($sub.status -eq "submitted") "submit did not queue for review"
  $claims = Api GET "/api/admin/tasks/claims" $null $atk
  $cid = (@($claims | Where-Object { $_.task_id -eq $bt.id })[0]).id
  Api POST "/api/tasks/claims/$cid/review" @{action="approve"} $atk | Out-Null
  $up1 = (Api GET "/api/shop/coins" $null $tk).points
  Assert($up1 - $up0 -eq 10) "bounty payout did not grant 10 points"
  # validations: non-admin cannot publish official task; over-afford bounty rejected
  $naRejected = $false
  try { Api POST "/api/tasks" @{kind="admin"; title="x"} $tk } catch { $naRejected = ((BadCode $_) -eq 403) }
  Assert($naRejected) "non-admin official task not rejected"
  $poorRejected = $false
  try { Api POST "/api/tasks" @{kind="bounty"; title="y"; reward_points=99999} $tk } catch { $poorRejected = ((BadCode $_) -eq 400) }
  Assert($poorRejected) "over-afford bounty not rejected" }
Step "tasks system auto-reward" {
  Assert($tk) "no token (earlier steps failed)"
  $sys = Api GET "/api/tasks?kind=system"
  $t = (@($sys | Where-Object { -not $_.my_status }) | Select-Object -First 1)
  if (-not $t) { Write-Host "    (system tasks all claimed - skip)"; return }
  Api POST "/api/tasks/$($t.id)/claim" @{} $tk | Out-Null
  $c0 = (Api GET "/api/shop/coins" $null $tk).coins
  $done = Api POST "/api/tasks/$($t.id)/complete" @{proof="e2e"} $tk
  Assert($done.status -eq "completed") "system task not auto-completed"
  $c1 = (Api GET "/api/shop/coins" $null $tk).coins
  Assert($c1 - $c0 -eq $t.reward_coins) "system task reward mismatch" }
Step "reports category + evidence" {
  Assert($tk -and $atk -and $pid1) "missing token or post (earlier steps failed)"
  # category label built from unicode escapes to keep this file ASCII (PS5.1 GBK hazard)
  $catAd = [regex]::Unescape('\u5e7f\u544a')
  $badCatRejected = $false
  try { Api POST "/api/reports" @{target_type="post"; target_id=$pid1; reason="not-a-category"} $tk } catch { $badCatRejected = ((BadCode $_) -eq 400) }
  Assert($badCatRejected) "invalid report category not rejected"
  $ev = "/static/uploads/e2e_ev_$Suffix.webp"
  $rp = Api POST "/api/reports" @{target_type="post"; target_id=$pid1; reason=$catAd; detail="e2e evidence"; evidence=@($ev, "http://evil.example/x.png")} $tk
  Assert($rp.id) "report create failed"
  $list = Api GET ("/api/reports?status=pending&category=" + [uri]::EscapeDataString($catAd)) $null $atk
  $row = @($list | Where-Object { $_.target_id -eq $pid1 })[0]
  Assert($row) "categorized report missing in admin queue"
  Assert(@($row.evidence).Count -eq 1) "evidence not parsed/filtered"
  Assert($row.evidence[0] -eq $ev) "evidence url mismatch" }
Step "data export" {
  Assert($tk -and $atk) "tokens missing (earlier steps failed)"
  $exp = Api GET "/api/auth/export" $null $tk
  Assert($exp.profile) "export missing profile"
  Assert($null -ne $exp.posts) "export missing posts"
  $out = HttpCodeSize "$BaseUrl/api/admin/export/users.csv" $atk
  $parts = "$out".Trim() -split '\s+'
  Assert($parts[0] -eq '200') "users.csv http=$($parts[0])"
  Assert([int]$parts[1] -gt 100) "users.csv too small"
  $outp = HttpCodeSize "$BaseUrl/api/admin/export/posts.csv" $atk
  $pp = "$outp".Trim() -split '\s+'
  Assert($pp[0] -eq '200') "posts.csv http=$($pp[0])"
  $forbidden = Invoke-CurlRaw @('-s','-o',$script:NullDev,'-w','%{http_code}','--max-time','25',"$BaseUrl/api/admin/export/users.csv",'-H',"Authorization: Bearer $tk")
  Assert("$forbidden" -eq '403') "csv export not admin-gated" }
Step "expose create + review + anon" {
  Assert($tk) "no token (earlier steps failed)"
  $ep = Api POST "/api/posts" @{title="e2e_ex$Suffix"; content="expose body"; post_type="expose"} $tk
  Assert($ep.post_id) "expose create no post_id"
  # pending: must NOT appear in the public expose feed
  $pub = Api GET "/api/posts?post_type=expose&limit=50"
  Assert(-not (@($pub.posts | Where-Object { $_.id -eq $ep.post_id }).Count)) "expose leaked pre-review"
  # admin sees it in review queue and approves
  $q = Api GET "/api/admin/review" $null $atk
  Assert(@($q.posts | Where-Object { $_.id -eq $ep.post_id }).Count -ge 1) "expose not in review queue"
  Api POST "/api/admin/review" @{post_id=$ep.post_id; action="approve"} $atk | Out-Null
  $pub = Api GET "/api/posts?post_type=expose&limit=50"
  $appr = @($pub.posts | Where-Object { $_.id -eq $ep.post_id })
  Assert($appr.Count -eq 1) "expose not public after approve"
  # forced anonymity: even the creator/admin see a masked name, never the username
  Assert($appr[0].author_name -ne $u) "expose author leaked"
  Assert($appr[0].author_name -ne $u2) "expose author leaked (u2)"
  Assert([string]$appr[0].author_name -ne "") "expose author empty" }
Step "expose images roundtrip" {
  Assert($tk -and $imgurl) "no token or no uploaded image (earlier steps failed)"
  $epi = Api POST "/api/posts" @{title="e2e_exi$Suffix"; content="with pics"; post_type="expose"; images=@($imgurl,$imgurl)} $tk
  Api POST "/api/admin/review" @{post_id=$epi.post_id; action="approve"} $atk | Out-Null
  $pub = Api GET "/api/posts?post_type=expose&limit=50"
  $row = @($pub.posts | Where-Object { $_.id -eq $epi.post_id })
  Assert($row.Count -eq 1) "expose-image post not public"
  $imgs = @($row[0].images)
  Assert($imgs.Count -eq 2) ("images not list of 2: " + ($row[0].images | ConvertTo-Json -Compress))
  Assert($imgs[0] -eq $imgurl) "image url mismatch" }
Step "forum boards + feed api" {
  $st = Api GET "/api/stations?limit=100"
  Assert(@($st).Count -ge 1) "no stations for forum"
  $feed = Api GET "/api/posts?limit=5&sort=newest"
  Assert($null -ne $feed.posts) "forum feed missing" }
Step "site config + ad toggle" {
  $cfg = Api GET "/api/site/config"
  Assert($null -ne $cfg) "no site config"
  Assert($null -ne $cfg.ad_enabled) "ad_enabled missing"
  $r = Api PUT "/api/admin/site/config" @{ad_enabled="0"} $atk
  Assert($null -ne $r) "admin site config failed"
  $cfg2 = Api GET "/api/site/config"
  Assert($cfg2.ad_enabled -ne $true) "ad toggle off not honored"
  Api PUT "/api/admin/site/config" @{ad_enabled="1"} $atk | Out-Null }
Step "vendored lucide served" {
  $out = HttpCodeSize "$BaseUrl/static/vendor/lucide.min.js"
  $parts = "$out".Trim() -split '\s+'
  Assert($parts[0] -eq '200') "lucide http=$($parts[0])"
  Assert([int]$parts[1] -gt 100000) "lucide bundle too small: $($parts[1])"
  $body = (Invoke-CurlRaw @('-s', '--max-time', '30', "$BaseUrl/static/vendor/lucide.min.js")) -join ''
  Assert($body -like "*createIcons*") "bundle missing createIcons"
  Assert($body -like "*1.46.0*") "bundle version not pinned" }

Write-Host ""
Write-Host ("== live E2E: pass={0} fail={1} ==" -f $pass, $fail) -ForegroundColor $(if($fail){'Red'}else{'Green'})
if ($fail) { $failList | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }; exit 1 } else { exit 0 }
