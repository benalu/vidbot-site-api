================================================================================
  VIDBOT ADMIN — ANALISIS INTEGRASI & REKOMENDASI PENINGKATAN
================================================================================
Dibuat: April 2026
Referensi: test-admin.html + semua file Go (router/, internal/, pkg/, middleware/)
================================================================================


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BAGIAN 1 — AUDIT INTEGRASI SAAT INI
  (Endpoint backend yang ADA vs yang SUDAH di-handle di frontend)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

SUDAH TERINTEGRASI (backend + frontend sama-sama ada):
  ✅ POST   /admin/auth/login              → doLogin()
  ✅ POST   /admin/auth/logout             → handleLogout()
  ✅ GET    /admin/auth/me                 → (dipakai saat showMainApp)
  ✅ POST   /admin/keys                    → createKey()
  ✅ GET    /admin/keys                    → loadKeys()
  ✅ DELETE /admin/keys/:keyHash           → revokeKey()
  ✅ POST   /admin/keys/:keyHash/topup     → topUpQuota()
  ✅ GET    /admin/keys/:keyHash/usage     → (dipakai di lookupKey tapi tidak ada UI-nya)
  ✅ GET    /admin/keys/:keyHash/reveal    → revealKey()
  ✅ GET    /admin/features                → loadFeatures()
  ✅ PUT    /admin/features/:group         → toggleFeature()
  ✅ PUT    /admin/features/:group/:plat   → toggleFeaturePlatform()
  ✅ GET    /admin/stats                   → loadStats()
  ✅ GET    /admin/stats/realtime          → (dipakai di loadStats)
  ✅ GET    /admin/stats/errors            → loadLogs()
  ✅ GET    /admin/system/health           → loadSettings()
  ✅ GET    /admin/system/redis            → loadRedisStats()
  ✅ GET    /admin/system/queue            → loadQueue()
  ✅ GET    /admin/system/sessions         → loadSessions()
  ✅ DELETE /admin/system/sessions/:id     → revokeSession()
  ✅ GET    /admin/providers               → loadProviders()
  ✅ PUT    /admin/providers/:g/:c         → updateProviderOrder()
  ✅ POST   /admin/providers/:g/:c/reset   → resetProviderOrder()
  ✅ GET    /admin/apps/:platform          → loadApps()
  ✅ POST   /admin/apps/:platform          → (addApp modal)
  ✅ POST   /admin/apps/:platform/bulk     → (bulkAdd modal)
  ✅ PATCH  /admin/apps/:platform/:slug    → (editApp modal)
  ✅ DELETE /admin/apps/:platform/:slug    → (deleteApp)
  ✅ GET    /admin/downloader/flac         → loadFlac()
  ✅ POST   /admin/downloader/flac         → (addFlac modal)
  ✅ POST   /admin/downloader/flac/bulk    → (bulkFlac modal)
  ✅ PATCH  /admin/downloader/flac/:id     → (editFlacMeta modal)
  ✅ PATCH  /admin/downloader/flac/:id/links → (editFlacLinks modal)
  ✅ DELETE /admin/downloader/flac/:id     → (deleteFlac)

BELUM TERINTEGRASI DI FRONTEND (backend ada, HTML belum):
  ❌ GET  /admin/keys?email=&name=&key_hash=  → Filter/search key dari server
     (Frontend hanya filter client-side, tidak kirim query param ke API)
  ❌ POST /admin/keys (lookup)                → lookupKey via api_key field
     (Ada handler LookupKey di backend tapi tidak ada UI-nya sama sekali)
  ❌ GET  /admin/apps/:platform/:slug         → Detail satu app (AdminGet)
  ❌ PATCH /admin/apps/:plat/:slug/versions/:id → Edit URL versi
  ❌ DELETE /admin/apps/:plat/:slug/versions/:id → Hapus satu versi
  ❌ GET  /admin/downloader/flac/:id          → Detail satu FLAC entry
  ❌ GET  /admin/stats?days=N&top=N&hourly=1  → Query param stats tidak dipakai
  ❌ GET  /admin/stats/realtime?minutes=N     → Minutes param diabaikan (hardcode)
  ❌ GET  /health (public)                    → Ada tapi hanya untuk checkAPIStatus,
     info detail (goroutines, worker, appstore dll) tidak ditampilkan

ENDPOINT DEMO/MOCK (data palsu di getDemoData, tidak benar-benar call API):
  ⚠ loadRequests() → 100% demo data statis, tidak ada endpoint backend nyata
    (ini memang wajar — tidak ada endpoint recent-requests di backend)
  ⚠ loadLogs() → Hanya call /admin/stats/errors, tidak ada real request log


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BAGIAN 2 — ENDPOINT BARU YANG DIREKOMENDASIKAN
  (Belum ada di backend, perlu dibuat)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Prioritas diberi tanda: [HIGH] [MED] [LOW]

────────────────────────────────────────────────────────────────────────────
2.1 [HIGH] GET /admin/stats/quota-alerts
────────────────────────────────────────────────────────────────────────────
TUJUAN: Deteksi cepat key yang hampir kehabisan quota sebelum user komplain.

Response:
  {
    "success": true,
    "data": {
      "critical": [          // quota_remaining < 10% dari quota total
        { "key_hash": "abc", "name": "Client X", "email": "x@...",
          "quota": 1000, "quota_used": 960, "quota_remaining": 40,
          "pct_used": 96.0 }
      ],
      "warning": [           // quota_remaining 10-25%
        { ... }
      ],
      "total_critical": 2,
      "total_warning": 5
    }
  }

Implementasi di handler.go (internal/admin/):
  Loop allKeys dari "apikeys:index", hitung pct_used,
  threshold critical=90%, warning=75%.

Kenapa penting: Tanpa ini admin harus scroll semua key manual untuk lihat
  mana yang hampir habis. User bisa tiba-tiba error QUOTA_EXCEEDED.

────────────────────────────────────────────────────────────────────────────
2.2 [HIGH] GET /admin/stats/platform-breakdown
────────────────────────────────────────────────────────────────────────────
TUJUAN: Lihat request count per platform untuk hari ini vs kemarin vs
  minggu lalu — langsung ketahuan platform mana yang tiba-tiba drop/spike.

Response:
  {
    "success": true,
    "data": {
      "content": {
        "spotify":   { "today": 120, "yesterday": 98,  "week_avg": 105, "trend": "+22%" },
        "tiktok":    { "today": 450, "yesterday": 380, "week_avg": 410, "trend": "+18%" },
        "instagram": { "today": 0,   "yesterday": 210, "week_avg": 195, "trend": "-100%" },
        ...
      },
      "vidhub": { ... },
      "convert": { ... }
    }
  }

Query SQL di pkg/stats/db.go:
  SELECT platform, grp,
    COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE) as today,
    COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE-1
                     AND created_at < CURRENT_DATE) as yesterday,
    COUNT(*) FILTER (WHERE created_at >= NOW()-INTERVAL'7 days') / 7.0 as week_avg
  FROM stats
  WHERE grp IN ('content','vidhub','convert','iptv','leakcheck','app')
    AND created_at >= NOW()-INTERVAL'8 days'
  GROUP BY grp, platform

Kenapa penting: Instagram drop to 0 langsung terlihat merah di dashboard
  tanpa perlu cek log satu per satu. Identifikasi masalah provider dalam
  hitungan detik.

────────────────────────────────────────────────────────────────────────────
2.3 [HIGH] GET /admin/keys/:keyHash/activity?days=7
────────────────────────────────────────────────────────────────────────────
TUJUAN: Lihat pola penggunaan satu key — kapan ramai, group apa yang paling
  banyak dipakai, error rate key ini.

Response:
  {
    "success": true,
    "data": {
      "key_hash": "...", "name": "Client X", "email": "x@...",
      "period_days": 7,
      "total_requests": 342,
      "daily_trend": [
        { "date": "2026-04-15", "count": 48 },
        { "date": "2026-04-16", "count": 52 },
        ...
      ],
      "by_group": { "content": 210, "vidhub": 98, "convert": 34 },
      "by_platform": { "tiktok": 180, "instagram": 30, "videb": 88, ... },
      "error_rate": { "EXTRACTION_FAILED": 12, "RATE_LIMIT_EXCEEDED": 3 },
      "last_seen": "2026-04-21T14:23:00Z"
    }
  }

Kenapa penting: Bisa deteksi key yang di-abuse (1000 req dalam 1 jam),
  atau key yang tidak aktif (0 request 7 hari terakhir tapi quota masih besar).

────────────────────────────────────────────────────────────────────────────
2.4 [MED] GET /admin/system/diagnostics
────────────────────────────────────────────────────────────────────────────
TUJUAN: Satu endpoint yang menggabungkan semua sinyal kesehatan sistem
  — Redis latency, queue, goroutine count, uptime — dalam satu call.
  Dashboard bisa poll ini setiap 10 detik tanpa banyak request.

Response:
  {
    "success": true,
    "checked_at": "...",
    "data": {
      "uptime": "2d 14h 33m",
      "goroutines": 47,
      "memory_mb": 128.4,
      "redis_latency_ms": 12,
      "redis_ok": true,
      "stats_db_ok": true,
      "hls_slots": { "current": 1, "max": 3 },
      "direct_slots": { "current": 4, "max": 10 },
      "worker_status": { "total": 3, "reachable": 3 },
      "iptv_channels": 42000,
      "leakcheck_entries": 5200000,
      "appstore_android": 350,
      "appstore_windows": 120,
      "alerts": [
        { "level": "warn", "msg": "Redis memory at 81% — approaching Upstash limit" }
      ]
    }
  }

Implementasi: Gabungkan data dari health.Handler, limiter, stats, cache
  dalam satu handler baru. Gunakan goroutine paralel seperti pola yang
  sudah ada di health/handler.go.

Kenapa penting: Saat ini untuk lihat status lengkap perlu 4 endpoint
  terpisah (/system/health, /system/redis, /system/queue, dan
  /stats/realtime). Dengan diagnostics bisa satu call.

────────────────────────────────────────────────────────────────────────────
2.5 [MED] POST /admin/keys/:keyHash/reset-quota
────────────────────────────────────────────────────────────────────────────
TUJUAN: Reset quota_used ke 0 tanpa harus mengubah quota limit.
  Berguna untuk billing cycle bulanan — reset di awal bulan.

Request body: {} (kosong)

Response:
  {
    "success": true,
    "message": "Quota usage reset for 'Client X'",
    "data": { "name": "Client X", "quota": 5000, "quota_used_before": 4823, "quota_used_now": 0 }
  }

Implementasi:
  cache.Set(ctx, fmt.Sprintf("apikeys:quota:%s", keyHash), "0", 0)

Kenapa penting: Saat ini tidak ada cara untuk reset usage. Admin harus
  revoke key lama + buat key baru — user kehilangan key-nya.

────────────────────────────────────────────────────────────────────────────
2.6 [MED] GET /admin/stats/errors/summary
────────────────────────────────────────────────────────────────────────────
TUJUAN: Ringkasan error yang lebih actionable — bukan hanya count per code
  tapi juga platform mana yang paling bermasalah dan jam berapa spike-nya.

Response:
  {
    "success": true,
    "data": {
      "period_hours": 24,
      "total_errors": 89,
      "error_rate_pct": 2.4,      // errors / total_requests * 100
      "top_errors": [
        { "code": "EXTRACTION_FAILED", "count": 34, "platforms": ["instagram","videb"] }
      ],
      "by_platform": {
        "instagram": { "errors": 21, "top_code": "EXTRACTION_FAILED" },
        "videb":     { "errors": 13, "top_code": "EXTRACTION_FAILED" }
      },
      "hourly_spike": [
        { "hour": "08", "count": 0 },
        { "hour": "14", "count": 27 },   // ← terlihat jelas ada spike jam 2 siang
        ...
      ]
    }
  }

Kenapa penting: Kalau instagram EXTRACTION_FAILED spike jam 14:00,
  kemungkinan besar provider/cookie expired di jam itu — bisa langsung
  investigasi tanpa harus filter log manual.

────────────────────────────────────────────────────────────────────────────
2.7 [MED] PUT /admin/system/rate-limits
────────────────────────────────────────────────────────────────────────────
TUJUAN: Update rate limit per group via API tanpa restart server.
  Saat ini rate limit hardcode di pkg/limiter/ratelimit.go.

Request:
  { "content": 20, "vidhub": 50, "convert": 30, "leakcheck": 10 }

Response:
  { "success": true, "message": "Rate limits updated", "data": { ... } }

Implementasi:
  Simpan ke Redis key "ratelimit:config:{group}" → integer.
  Update CheckRateLimit() untuk baca dari Redis dulu, fallback ke hardcode.

Kenapa penting: Kalau ada serangan atau provider sedang maintainance,
  admin bisa turunkan rate limit tanpa deploy ulang.

────────────────────────────────────────────────────────────────────────────
2.8 [MED] GET /admin/system/cache-stats
────────────────────────────────────────────────────────────────────────────
TUJUAN: Lihat berapa banyak request yang kena cache hit vs miss — untuk
  tahu apakah cache TTL perlu di-tune.

Response:
  {
    "success": true,
    "data": {
      "shortlinks": { "total": 1243, "active": 891 },
      "content_cache": {
        "tiktok": 234, "instagram": 89, "spotify": 512,
        "twitter": 67, "threads": 23
      },
      "vidhub_cache": {
        "videb": 445, "vidoy": 123, "vidbos": 234
      },
      "app_shortlinks": 567,
      "dl_shortlinks": 234,
      "total_cached_items": 3445,
      "estimated_memory_saved": "~12MB"
    }
  }

Implementasi: Pakai cache.CountKeys() yang sudah ada dengan pattern
  berbeda per prefix.

────────────────────────────────────────────────────────────────────────────
2.9 [LOW] DELETE /admin/system/cache/:prefix
────────────────────────────────────────────────────────────────────────────
TUJUAN: Flush cache untuk service tertentu saat ada bug atau data stale.
  Misal flush cache instagram saja tanpa restart.

Path param prefix: "content:instagram", "vidhub:videb", "sl", "app:sl"

Response:
  { "success": true, "message": "Flushed 234 keys with prefix 'content:instagram:*'" }

Implementasi: Scan + Del dengan pattern, pakai pipeline Redis untuk
  batch delete.

PENTING: Batasi prefix yang boleh dihapus (whitelist) agar tidak
  bisa hapus "apikeys:*" atau "admin:session:*" secara tidak sengaja.

────────────────────────────────────────────────────────────────────────────
2.10 [LOW] GET /admin/system/goroutines
────────────────────────────────────────────────────────────────────────────
TUJUAN: Monitor goroutine leak — kalau goroutine count terus naik padahal
  traffic normal, berarti ada goroutine yang tidak pernah selesai.

Response:
  {
    "success": true,
    "data": {
      "count": 47,
      "baseline": 35,
      "delta": "+12",
      "alert": false,
      "hls_sessions_active": 2,
      "note": "Normal range: 35-80 depending on concurrent HLS sessions"
    }
  }

Implementasi: runtime.NumGoroutine() + track baseline di startup.

────────────────────────────────────────────────────────────────────────────
2.11 [LOW] POST /admin/keys/bulk-topup
────────────────────────────────────────────────────────────────────────────
TUJUAN: Top up quota semua key aktif sekaligus di awal bulan.

Request:
  { "amount": 5000, "only_active": true, "email_filter": "@company.com" }

Response:
  {
    "success": true,
    "data": { "updated": 18, "skipped": 1, "total_added": 90000 }
  }

Kenapa penting: Saat ada 20+ key, top up satu per satu sangat melelahkan.

────────────────────────────────────────────────────────────────────────────
2.12 [LOW] GET /admin/leakcheck/stats
────────────────────────────────────────────────────────────────────────────
TUJUAN: Status lengkap leakcheck database — entry count, search latency,
  cache hit rate.

Response:
  {
    "success": true,
    "data": {
      "status": "ok",
      "total_entries": 5234789,
      "db_size_mb": 2847.3,
      "search_cache_entries": 234,
      "avg_search_ms": 8,
      "last_reload": "2026-04-15T03:00:00Z"
    }
  }

Implementasi: Tambahkan method GetStats() ke pkg/leakcheck/store.go.


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BAGIAN 3 — PERUBAHAN test-admin.html (Pattern Lama vs Baru)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Format: [LOKASI] Deskripsi perubahan
  LAMA:  ... kode lama ...
  BARU:  ... kode baru ...

────────────────────────────────────────────────────────────────────────────
3.1 Tambah sidebar item "Alerts" (untuk endpoint 2.1 & 2.6)
────────────────────────────────────────────────────────────────────────────
[LOKASI: nav di sidebar, setelah item "Dashboard", sebelum "API Keys"]

LAMA:
      <a class="sidebar-link flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm cursor-pointer mb-1" style="color:#6b7db3" onclick="navigate('keys')">
        <svg ...key icon.../>
        API Keys
        <span ... id="keyCount">0</span>
      </a>

BARU (tambahkan SEBELUM baris di atas):
      <a class="sidebar-link flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm cursor-pointer mb-1" style="color:#6b7db3" onclick="navigate('alerts')">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
        Alerts
        <span class="ml-auto text-xs px-1.5 py-0.5 rounded hidden" style="background:rgba(239,68,68,0.15); color:#f87171;" id="alertBadge">0</span>
      </a>

────────────────────────────────────────────────────────────────────────────
3.2 Tambah section "Alerts" (HTML section baru)
────────────────────────────────────────────────────────────────────────────
[LOKASI: setelah </section> penutup section-dashboard, sebelum section-keys]

LAMA: (tidak ada)

BARU (tambahkan section HTML baru):
    <!-- ===== ALERTS ===== -->
    <section class="section" id="section-alerts">
      <div class="flex items-center justify-between mb-5">
        <div>
          <h2 class="text-sm font-semibold text-white">Alerts & Diagnostics</h2>
          <p class="text-xs mt-0.5" style="color:#4a5578;">Quota warnings, platform drops, and system anomalies</p>
        </div>
        <button class="btn-ghost px-3 py-1.5 text-xs" onclick="loadAlerts()">Refresh</button>
      </div>
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
        <!-- Quota Alerts -->
        <div class="rounded-xl p-5" style="background:#0f1117; border:1px solid #1e2540;">
          <div class="text-sm font-semibold text-white mb-3">Quota Alerts</div>
          <div id="quotaAlertsList" class="space-y-2">
            <div class="shimmer h-10 rounded-lg"></div>
          </div>
        </div>
        <!-- Platform Health -->
        <div class="rounded-xl p-5" style="background:#0f1117; border:1px solid #1e2540;">
          <div class="text-sm font-semibold text-white mb-3">Platform vs Yesterday</div>
          <div id="platformBreakdown" class="space-y-2">
            <div class="shimmer h-10 rounded-lg"></div>
          </div>
        </div>
      </div>
      <!-- Error Summary -->
      <div class="rounded-xl p-5" style="background:#0f1117; border:1px solid #1e2540;">
        <div class="text-sm font-semibold text-white mb-3">Error Summary (24h)</div>
        <div id="errorSummaryFull" class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="shimmer h-24 rounded-lg"></div>
          <div class="shimmer h-24 rounded-lg"></div>
        </div>
      </div>
    </section>

────────────────────────────────────────────────────────────────────────────
3.3 Tambah navigate('alerts') di fungsi navigate()
────────────────────────────────────────────────────────────────────────────
[LOKASI: dalam fungsi navigate(), di objek `titles`]

LAMA:
  const titles = {
    dashboard: ['Dashboard', 'System overview & metrics'],
    keys: ['API Keys', 'Manage access credentials'],
    ...
    settings: ['Settings', 'System configuration & sessions'],
  };

BARU (tambah entry 'alerts'):
  const titles = {
    dashboard: ['Dashboard', 'System overview & metrics'],
    alerts: ['Alerts', 'Quota warnings and platform diagnostics'],
    keys: ['API Keys', 'Manage access credentials'],
    ...
    settings: ['Settings', 'System configuration & sessions'],
  };

────────────────────────────────────────────────────────────────────────────
3.4 Tambah pemanggilan loadAlerts() di navigate()
────────────────────────────────────────────────────────────────────────────
[LOKASI: dalam fungsi navigate(), blok if-else yang memanggil load functions]

LAMA:
  if (section === 'dashboard') { loadStats(); loadRequests(); }
  if (section === 'keys') loadKeys();
  if (section === 'endpoints') loadEndpoints();

BARU (tambah baris setelah dashboard):
  if (section === 'dashboard') { loadStats(); loadRequests(); }
  if (section === 'alerts') loadAlerts();
  if (section === 'keys') loadKeys();
  if (section === 'endpoints') loadEndpoints();

────────────────────────────────────────────────────────────────────────────
3.5 Tambah fungsi loadAlerts() di JavaScript
────────────────────────────────────────────────────────────────────────────
[LOKASI: tambahkan fungsi baru setelah fungsi loadLogs()]

LAMA: (tidak ada)

BARU (tambahkan fungsi baru):
async function loadAlerts() {
  // 1. Quota alerts
  const quotaData = await apiCall('/admin/stats/quota-alerts');
  const quotaEl = document.getElementById('quotaAlertsList');
  if (quotaData?.success) {
    const critical = quotaData.data?.critical || [];
    const warning = quotaData.data?.warning || [];
    const all = [
      ...critical.map(k => ({ ...k, level: 'critical' })),
      ...warning.map(k => ({ ...k, level: 'warning' }))
    ];
    // Update badge di sidebar
    const badge = document.getElementById('alertBadge');
    if (critical.length > 0) {
      badge.textContent = critical.length;
      badge.classList.remove('hidden');
    } else {
      badge.classList.add('hidden');
    }
    quotaEl.innerHTML = all.length ? all.map(k => {
      const color = k.level === 'critical' ? '#f87171' : '#fb923c';
      const bg = k.level === 'critical' ? 'rgba(239,68,68,0.08)' : 'rgba(251,146,60,0.08)';
      return `<div class="p-3 rounded-lg" style="background:${bg}; border:1px solid ${color}33;">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-xs font-medium text-white">${k.name}</div>
            <div class="text-xs" style="color:#6b7db3;">${k.email}</div>
          </div>
          <div class="text-right">
            <div class="text-xs font-mono" style="color:${color};">${k.pct_used.toFixed(1)}% used</div>
            <div class="text-xs" style="color:#4a5578;">${k.quota_remaining.toLocaleString()} remaining</div>
          </div>
        </div>
      </div>`;
    }).join('') : '<div class="text-xs text-center py-4 rounded-lg" style="color:#34d399; background:rgba(52,211,153,0.05);">✓ All keys have sufficient quota</div>';
  }

  // 2. Platform breakdown (today vs yesterday)
  const platData = await apiCall('/admin/stats/platform-breakdown');
  const platEl = document.getElementById('platformBreakdown');
  if (platData?.success) {
    const rows = [];
    Object.entries(platData.data || {}).forEach(([group, platforms]) => {
      Object.entries(platforms).forEach(([platform, stats]) => {
        const trendNum = parseFloat(stats.trend);
        const isDown = trendNum < -50;
        rows.push({ group, platform, ...stats, isDown });
      });
    });
    // sort: platforms yang drop paling banyak di atas
    rows.sort((a, b) => parseFloat(a.trend) - parseFloat(b.trend));
    platEl.innerHTML = rows.slice(0, 8).map(r => {
      const color = r.isDown ? '#f87171' : parseFloat(r.trend) > 20 ? '#34d399' : '#6b7db3';
      return `<div class="flex items-center justify-between text-xs py-1.5" style="border-bottom:1px solid #1e2540;">
        <div><span class="text-white">${r.group}/${r.platform}</span></div>
        <div class="flex items-center gap-3">
          <span style="color:#6b7db3;">${r.today} today</span>
          <span style="color:${color}; font-weight:600;">${r.trend}</span>
        </div>
      </div>`;
    }).join('') || '<div class="text-xs" style="color:#4a5578;">No data</div>';
  }

  // 3. Error summary
  const errData = await apiCall('/admin/stats/errors/summary');
  const errEl = document.getElementById('errorSummaryFull');
  if (errData?.success) {
    const d = errData.data;
    errEl.innerHTML = `
      <div>
        <div class="text-xs font-medium text-white mb-2">Error Rate</div>
        <div class="text-3xl font-bold font-display" style="color:${d.error_rate_pct > 5 ? '#f87171' : d.error_rate_pct > 2 ? '#fb923c' : '#34d399'};">
          ${d.error_rate_pct?.toFixed(1)}%
        </div>
        <div class="text-xs mt-1" style="color:#4a5578;">${d.total_errors} errors / 24h</div>
        <div class="mt-3 space-y-1">
          ${(d.top_errors || []).map(e => `
            <div class="flex justify-between text-xs">
              <span class="font-mono" style="color:#f87171;">${e.code}</span>
              <span style="color:#6b7db3;">${e.count}×</span>
            </div>`).join('')}
        </div>
      </div>
      <div>
        <div class="text-xs font-medium text-white mb-2">By Platform</div>
        ${Object.entries(d.by_platform || {}).map(([p, s]) => `
          <div class="flex justify-between text-xs py-1" style="border-bottom:1px solid #1e2540;">
            <span style="color:#c9d1e0;">${p}</span>
            <span style="color:#f87171;">${s.errors} (${s.top_code})</span>
          </div>`).join('')}
      </div>`;
  }
}

────────────────────────────────────────────────────────────────────────────
3.6 Tambah "Reset Quota" di action button keys
────────────────────────────────────────────────────────────────────────────
[LOKASI: dalam fungsi renderKeys(), di dalam map(), di blok <td> actions]

LAMA:
          <button class="btn-ghost px-2 py-1 text-xs" onclick="showTopup('${k.key_hash}', '${k.name}')" title="Top Up Quota">+Q</button>
          <button class="btn-ghost px-2 py-1 text-xs" onclick="revealKey('${k.key_hash}')" title="Reveal Plain Key" style="color:#fb923c;">👁</button>
          ${k.active ? `<button class="btn-danger px-2 py-1 text-xs" onclick="revokeKey('${k.key_hash}', '${k.name}')">Revoke</button>` : '<span class="text-xs" style="color:#4a5578;">—</span>'}

BARU (tambah tombol Reset Quota):
          <button class="btn-ghost px-2 py-1 text-xs" onclick="showTopup('${k.key_hash}', '${k.name}')" title="Top Up Quota">+Q</button>
          <button class="btn-ghost px-2 py-1 text-xs" onclick="showKeyActivity('${k.key_hash}', '${k.name}')" title="View Activity" style="color:#38bdf8;">📊</button>
          <button class="btn-ghost px-2 py-1 text-xs" onclick="resetQuota('${k.key_hash}', '${k.name}')" title="Reset Quota Usage" style="color:#fb923c;">↺</button>
          <button class="btn-ghost px-2 py-1 text-xs" onclick="revealKey('${k.key_hash}')" title="Reveal Plain Key" style="color:#fb923c;">👁</button>
          ${k.active ? `<button class="btn-danger px-2 py-1 text-xs" onclick="revokeKey('${k.key_hash}', '${k.name}')">Revoke</button>` : '<span class="text-xs" style="color:#4a5578;">—</span>'}

────────────────────────────────────────────────────────────────────────────
3.7 Tambah fungsi resetQuota() dan showKeyActivity()
────────────────────────────────────────────────────────────────────────────
[LOKASI: tambahkan setelah fungsi revealKey()]

LAMA: (tidak ada)

BARU:
async function resetQuota(keyHash, name) {
  if (!confirm(`Reset quota usage for "${name}"? This will set usage back to 0.`)) return;
  const data = await apiCall(`/admin/keys/${keyHash}/reset-quota`, { method: 'POST', body: '{}' });
  if (data?.success) {
    toast(`Quota usage reset for ${name}`, 'success');
    loadKeys();
  } else {
    toast('Failed to reset quota', 'error');
  }
}

async function showKeyActivity(keyHash, name) {
  const data = await apiCall(`/admin/keys/${keyHash}/activity?days=7`);
  if (!data?.success) { toast('Could not load activity', 'error'); return; }
  const d = data.data;
  document.getElementById('keyActivityTitle').textContent = `Activity: ${name}`;
  document.getElementById('keyActivityContent').innerHTML = `
    <div class="grid grid-cols-3 gap-3 mb-4">
      <div class="p-3 rounded-lg text-center" style="background:#141720;">
        <div class="text-xl font-bold text-white">${d.total_requests.toLocaleString()}</div>
        <div class="text-xs" style="color:#4a5578;">7-day requests</div>
      </div>
      <div class="p-3 rounded-lg text-center" style="background:#141720;">
        <div class="text-xl font-bold" style="color:${d.error_rate?.EXTRACTION_FAILED > 10 ? '#f87171' : '#34d399'};">${Object.values(d.error_rate||{}).reduce((a,b)=>a+b,0)}</div>
        <div class="text-xs" style="color:#4a5578;">errors</div>
      </div>
      <div class="p-3 rounded-lg text-center" style="background:#141720;">
        <div class="text-xl font-bold text-white">${d.last_seen ? new Date(d.last_seen).toLocaleDateString() : 'N/A'}</div>
        <div class="text-xs" style="color:#4a5578;">last seen</div>
      </div>
    </div>
    <div class="text-xs font-medium text-white mb-2">Usage by Group</div>
    ${Object.entries(d.by_group||{}).map(([g,c]) => `
      <div class="flex justify-between text-xs py-1" style="border-bottom:1px solid #1e2540;">
        <span style="color:#c9d1e0;">${g}</span>
        <span style="color:#818cf8;">${c.toLocaleString()}</span>
      </div>`).join('')}
  `;
  document.getElementById('keyActivityModal').classList.add('show');
}

────────────────────────────────────────────────────────────────────────────
3.8 Tambah modal keyActivityModal
────────────────────────────────────────────────────────────────────────────
[LOKASI: tambahkan setelah modal revealKeyModal, sebelum tag </body>]

LAMA: (tidak ada)

BARU:
<!-- ===== KEY ACTIVITY MODAL ===== -->
<div class="modal-overlay" id="keyActivityModal">
  <div class="rounded-xl w-full max-w-md mx-4 p-6" style="background:#0f1117; border:1px solid #1e2540; max-height:80vh; overflow-y:auto;">
    <div class="flex items-center justify-between mb-4">
      <div class="text-sm font-bold text-white" id="keyActivityTitle">Key Activity</div>
      <button class="btn-ghost px-2 py-1.5" onclick="closeModal('keyActivityModal')">✕</button>
    </div>
    <div id="keyActivityContent"></div>
    <button class="btn-ghost w-full mt-4" onclick="closeModal('keyActivityModal')">Close</button>
  </div>
</div>

────────────────────────────────────────────────────────────────────────────
3.9 Tambah "Diagnostics" card di section-settings
────────────────────────────────────────────────────────────────────────────
[LOKASI: dalam section-settings, setelah card "Rate Limit Configuration"]

LAMA:
        <!-- Rate Limits (static config) -->
        <div class="rounded-xl p-5 lg:col-span-2" style="background:#0f1117; border:1px solid #1e2540;">
          ... rate limit config ...
        </div>
      </div>
    </section>

BARU (tambahkan SETELAH card Rate Limits, SEBELUM </div></section>):
        <!-- System Diagnostics -->
        <div class="rounded-xl p-5 lg:col-span-2" style="background:#0f1117; border:1px solid #1e2540;">
          <div class="flex items-center justify-between mb-4">
            <div class="text-sm font-semibold text-white">System Diagnostics</div>
            <button class="btn-ghost px-2 py-1 text-xs" onclick="loadDiagnostics()">Refresh</button>
          </div>
          <div id="diagnosticsContent" class="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div class="shimmer h-16 rounded-lg"></div>
            <div class="shimmer h-16 rounded-lg"></div>
            <div class="shimmer h-16 rounded-lg"></div>
            <div class="shimmer h-16 rounded-lg"></div>
          </div>
          <div id="diagnosticsAlerts" class="mt-3 space-y-2"></div>
        </div>

────────────────────────────────────────────────────────────────────────────
3.10 Tambah fungsi loadDiagnostics() dan panggil dari loadSettings()
────────────────────────────────────────────────────────────────────────────
[LOKASI: 1) tambah fungsi baru setelah loadQueue(); 2) ubah loadSettings()]

FUNGSI BARU (tambahkan setelah loadQueue()):
async function loadDiagnostics() {
  const data = await apiCall('/admin/system/diagnostics');
  if (!data?.success) return;
  const d = data.data;
  document.getElementById('diagnosticsContent').innerHTML = [
    { label: 'Uptime',       value: d.uptime,              color: 'white' },
    { label: 'Goroutines',   value: d.goroutines,           color: d.goroutines > 200 ? '#f87171' : 'white' },
    { label: 'Memory',       value: d.memory_mb?.toFixed(1) + 'MB', color: 'white' },
    { label: 'Redis Latency',value: d.redis_latency_ms + 'ms',   color: d.redis_latency_ms > 100 ? '#fb923c' : '#34d399' },
  ].map(item => `
    <div class="p-3 rounded-lg" style="background:#141720; border:1px solid #1e2540;">
      <div class="text-xs" style="color:#4a5578;">${item.label}</div>
      <div class="text-lg font-bold font-display mt-1" style="color:${item.color};">${item.value}</div>
    </div>`).join('');

  const alerts = d.alerts || [];
  document.getElementById('diagnosticsAlerts').innerHTML = alerts.map(a => `
    <div class="p-2.5 rounded-lg text-xs flex items-center gap-2" style="background:rgba(251,146,60,0.08); border:1px solid rgba(251,146,60,0.2); color:#fb923c;">
      <span>⚠</span>${a.msg}
    </div>`).join('');
}

UBAH loadSettings() — LAMA:
async function loadSettings() {
  loadHealth();
  loadRedisStats();
  loadQueue();
  loadSessions();
}

BARU (tambah loadDiagnostics()):
async function loadSettings() {
  loadHealth();
  loadRedisStats();
  loadQueue();
  loadSessions();
  loadDiagnostics();
}

────────────────────────────────────────────────────────────────────────────
3.11 Perbaiki loadRequests() — ganti dummy data dengan data real
────────────────────────────────────────────────────────────────────────────
KONTEKS: Saat ini loadRequests() memakai demoRequests statis.
  Backend tidak punya endpoint request log.
  Solusi: tampilkan "Recent Errors" dari /admin/stats/errors sebagai gantinya,
  atau sembunyikan tabel dan ganti dengan info yang lebih berguna.

[LOKASI: di dalam section-dashboard, bagian "Recent Requests" + fungsi loadRequests()]

UBAH judul header tabel di HTML — LAMA:
              <div class="text-sm font-semibold text-white">Recent Requests</div>
              <div class="text-xs mt-0.5" style="color:#4a5578;">Latest API activity</div>

BARU:
              <div class="text-sm font-semibold text-white">Recent Errors</div>
              <div class="text-xs mt-0.5" style="color:#4a5578;">Last 20 errors from all groups</div>

UBAH header tabel — LAMA:
                <th ... >METHOD</th>
                <th ... >PATH</th>
                <th ... >STATUS</th>
                <th ... >LATENCY</th>
                <th ... >IP</th>
                <th ... >TIME</th>

BARU:
                <th ... >GROUP</th>
                <th ... >PLATFORM</th>
                <th ... >ERROR CODE</th>
                <th ... >KEY</th>
                <th ... >TIME</th>

UBAH fungsi loadRequests() — LAMA:
async function loadRequests() {
  const tbody = document.getElementById('requestTable');
  const requests = demoRequests.map(r => ({ ...r, ts: new Date().toLocaleTimeString(...) }));
  tbody.innerHTML = requests.map((r, i) => { ... }).join('');
}

BARU:
async function loadRequests() {
  const tbody = document.getElementById('requestTable');
  const data = await apiCall('/admin/stats/errors?limit=20&hours=24');
  const recent = data?.data?.recent || [];
  if (!recent.length) {
    tbody.innerHTML = '<tr><td colspan="5" class="px-5 py-8 text-center" style="color:#4a5578;">No recent errors — looking good! ✓</td></tr>';
    return;
  }
  tbody.innerHTML = recent.map((r, i) => {
    const gc = { content:'#a78bfa', vidhub:'#38bdf8', convert:'#fb923c', iptv:'#34d399', leakcheck:'#f472b6', app:'#818cf8' }[r.group] || '#6b7db3';
    const ts = r.created_at ? new Date(r.created_at).toLocaleTimeString('en-US', {hour12:false}) : '—';
    return `<tr class="table-row fadeIn" style="animation-delay:${i*30}ms">
      <td class="px-5 py-3"><span class="method-badge" style="background:${gc}18; color:${gc};">${r.group}</span></td>
      <td class="px-5 py-3 font-mono text-xs text-white">${r.platform || '—'}</td>
      <td class="px-5 py-3"><span class="status-badge" style="background:rgba(239,68,68,0.1); color:#f87171;">${r.code}</span></td>
      <td class="px-5 py-3 font-mono" style="color:#4a5578;">${(r.key_hash||'').slice(0,8)}•••</td>
      <td class="px-5 py-3" style="color:#4a5578;">${ts}</td>
    </tr>`;
  }).join('');
}

────────────────────────────────────────────────────────────────────────────
3.12 Tambah cache stats di section-settings
────────────────────────────────────────────────────────────────────────────
[LOKASI: dalam section-settings, setelah card "Active Sessions"]

LAMA:
        <!-- Rate Limits (static config) -->
        <div class="rounded-xl p-5 lg:col-span-2" ...>

BARU (tambahkan SEBELUM Rate Limits):
        <!-- Cache Stats -->
        <div class="rounded-xl p-5" style="background:#0f1117; border:1px solid #1e2540;">
          <div class="flex items-center justify-between mb-4">
            <div class="text-sm font-semibold text-white">Cache Stats</div>
            <button class="btn-ghost px-2 py-1 text-xs" onclick="loadCacheStats()">Refresh</button>
          </div>
          <div id="cacheStatsList" class="space-y-1.5">
            <div class="shimmer h-6 rounded"></div>
            <div class="shimmer h-6 rounded"></div>
          </div>
        </div>

Dan tambahkan fungsi loadCacheStats() setelah loadDiagnostics():

async function loadCacheStats() {
  const data = await apiCall('/admin/system/cache-stats');
  if (!data?.success) return;
  const d = data.data;
  const el = document.getElementById('cacheStatsList');
  const flat = [];
  if (d.content_cache) Object.entries(d.content_cache).forEach(([k,v]) => flat.push([`content:${k}`, v]));
  if (d.vidhub_cache) Object.entries(d.vidhub_cache).forEach(([k,v]) => flat.push([`vidhub:${k}`, v]));
  flat.push(['shortlinks', d.shortlinks?.active || 0]);
  flat.push(['app shortlinks', d.app_shortlinks || 0]);
  flat.push(['dl shortlinks', d.dl_shortlinks || 0]);
  el.innerHTML = flat.map(([k,v]) => `
    <div class="flex justify-between text-xs py-1" style="border-bottom:1px solid #1a1f2e;">
      <span class="font-mono" style="color:#6b7db3;">${k}</span>
      <span style="color:#c9d1e0;">${v} keys</span>
    </div>`).join('') +
    `<div class="text-xs mt-2 pt-2" style="border-top:1px solid #1e2540; color:#4a5578;">
      Est. memory saved: <span style="color:#818cf8;">${d.estimated_memory_saved || 'N/A'}</span>
    </div>`;
}

Dan tambahkan loadCacheStats() ke dalam loadSettings():

LAMA:
async function loadSettings() {
  loadHealth();
  loadRedisStats();
  loadQueue();
  loadSessions();
  loadDiagnostics();
}

BARU:
async function loadSettings() {
  loadHealth();
  loadRedisStats();
  loadQueue();
  loadSessions();
  loadDiagnostics();
  loadCacheStats();
}

────────────────────────────────────────────────────────────────────────────
3.13 Auto-refresh Alerts badge di dashboard load
────────────────────────────────────────────────────────────────────────────
[LOKASI: dalam fungsi loadStats(), di bagian akhir fungsi]

TUJUAN: Setiap kali dashboard dibuka, cek apakah ada quota alert
  dan tampilkan badge merah di sidebar item "Alerts".

LAMA:
  // (loadStats tidak update badge)

BARU (tambahkan di akhir fungsi loadStats(), setelah semua update):
  // Cek quota alerts untuk badge sidebar
  apiCall('/admin/stats/quota-alerts').then(d => {
    const badge = document.getElementById('alertBadge');
    if (d?.data?.critical?.length > 0) {
      badge.textContent = d.data.critical.length;
      badge.classList.remove('hidden');
    } else {
      badge.classList.add('hidden');
    }
  });

────────────────────────────────────────────────────────────────────────────
3.14 Fix: server-side filter di loadKeys()
────────────────────────────────────────────────────────────────────────────
KONTEKS: Saat ini filterKeys() hanya filter client-side dari allKeys.
  Backend sudah support query param ?email=&name=&active=.
  Ini tidak masalah untuk jumlah key sedikit, tapi kalau ratusan key
  sebaiknya pakai server-side filter.

[LOKASI: fungsi loadKeys()]

LAMA:
async function loadKeys() {
  const data = await apiCall('/admin/keys');
  allKeys = data?.data || [];
  document.getElementById('keyCount').textContent = allKeys.filter(k => k.active).length;
  renderKeys(allKeys);
}

BARU (tambah support query filter — backwards compatible):
async function loadKeys(serverFilter = false) {
  let url = '/admin/keys';
  if (serverFilter) {
    const q = document.getElementById('keySearch').value.toLowerCase();
    const f = document.getElementById('keyFilter').value;
    const params = new URLSearchParams();
    if (q && q.includes('@')) params.set('email', q);
    else if (q) params.set('name', q);
    if (f === 'active') params.set('active', 'true');
    if (f === 'inactive') params.set('active', 'false');
    if (params.toString()) url += '?' + params.toString();
  }
  const data = await apiCall(url);
  allKeys = data?.data || [];
  document.getElementById('keyCount').textContent = allKeys.filter(k => k.active).length;
  renderKeys(allKeys);
}


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BAGIAN 4 — ENDPOINT BACKEND BARU: PANDUAN IMPLEMENTASI Go
  (Diurutkan dari yang paling mudah dibuat)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Semua handler baru masuk ke: internal/admin/handler.go
Semua route baru masuk ke: router/admin.go

────────────────────────────────────────────────────────────────────────────
4.1 POST /admin/keys/:keyHash/reset-quota  (paling mudah, ~15 baris)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go, dalam adminGroup:
  adminGroup.POST("/keys/:keyHash/reset-quota", adminHandler.ResetQuota)

Di internal/admin/handler.go:
  func (h *Handler) ResetQuota(c *gin.Context) {
      keyHash := c.Param("keyHash")
      ctx := context.Background()
      redisKey := fmt.Sprintf("apikeys:%s", keyHash)
      raw, err := cache.Get(ctx, redisKey)
      if err != nil {
          response.AdminNotFound(c, "API key not found.")
          return
      }
      var data apikey.Data
      json.Unmarshal([]byte(raw), &data)

      quotaKey := fmt.Sprintf("apikeys:quota:%s", keyHash)
      quotaStr, _ := cache.Get(ctx, quotaKey)
      oldUsed := 0
      fmt.Sscanf(quotaStr, "%d", &oldUsed)

      cache.Set(ctx, quotaKey, "0", 0)

      c.JSON(http.StatusOK, gin.H{
          "success": true,
          "message": fmt.Sprintf("Quota usage reset for '%s'", data.Name),
          "data": gin.H{
              "name":              data.Name,
              "quota":             data.Quota,
              "quota_used_before": oldUsed,
              "quota_used_now":    0,
          },
      })
  }

────────────────────────────────────────────────────────────────────────────
4.2 GET /admin/stats/quota-alerts  (~35 baris)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go:
  adminGroup.GET("/stats/quota-alerts", adminHandler.GetQuotaAlerts)

Di internal/admin/handler.go:
  func (h *Handler) GetQuotaAlerts(c *gin.Context) {
      ctx := context.Background()
      keyHashes, err := cache.SMembers(ctx, "apikeys:index")
      if err != nil {
          c.JSON(http.StatusOK, adminResponse{Success: true, Data: gin.H{
              "critical": []gin.H{}, "warning": []gin.H{},
              "total_critical": 0, "total_warning": 0,
          }})
          return
      }

      critical := []gin.H{}
      warning := []gin.H{}

      for _, keyHash := range keyHashes {
          raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
          if err != nil { continue }
          var data apikey.Data
          json.Unmarshal([]byte(raw), &data)
          if !data.Active || data.Quota == 0 { continue }

          quotaStr, _ := cache.Get(ctx, fmt.Sprintf("apikeys:quota:%s", keyHash))
          used := 0
          fmt.Sscanf(quotaStr, "%d", &used)
          remaining := data.Quota - used
          pct := float64(used) / float64(data.Quota) * 100

          entry := gin.H{
              "key_hash":        keyHash,
              "name":            data.Name,
              "email":           data.Email,
              "quota":           data.Quota,
              "quota_used":      used,
              "quota_remaining": remaining,
              "pct_used":        math.Round(pct*10) / 10,
          }

          if pct >= 90 {
              critical = append(critical, entry)
          } else if pct >= 75 {
              warning = append(warning, entry)
          }
      }

      c.JSON(http.StatusOK, adminResponse{Success: true, Data: gin.H{
          "critical": critical, "warning": warning,
          "total_critical": len(critical), "total_warning": len(warning),
      }})
  }

  // Tambah import: "math"

────────────────────────────────────────────────────────────────────────────
4.3 GET /admin/keys/:keyHash/activity?days=7  (~25 baris)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go:
  adminGroup.GET("/keys/:keyHash/activity", adminHandler.GetKeyActivity)

Di internal/admin/handler.go:
  func (h *Handler) GetKeyActivity(c *gin.Context) {
      keyHash := c.Param("keyHash")
      days := 7
      if d := c.Query("days"); d != "" {
          fmt.Sscanf(d, "%d", &days)
          if days < 1 || days > 90 { days = 7 }
      }

      ctx := context.Background()
      raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
      if err != nil {
          response.AdminNotFound(c, "API key not found.")
          return
      }
      var data apikey.Data
      json.Unmarshal([]byte(raw), &data)

      usagePerGroup := stats.GetKeyUsageByGroup(keyHash)
      dailyTrend := stats.GetKeyDailyStats(keyHash, days)  // fungsi baru di pkg/stats
      lastSeen := stats.GetKeyLastSeen(keyHash)              // fungsi baru di pkg/stats
      errorRate := stats.GetKeyErrorRate(keyHash, days)     // fungsi baru di pkg/stats

      c.JSON(http.StatusOK, adminResponse{Success: true, Data: gin.H{
          "key_hash":     keyHash,
          "name":         data.Name,
          "email":        data.Email,
          "period_days":  days,
          "by_group":     usagePerGroup,
          "daily_trend":  dailyTrend,
          "error_rate":   errorRate,
          "last_seen":    lastSeen,
      }})
  }

Di pkg/stats/db.go, tambahkan fungsi pendukung:
  func GetKeyDailyStats(keyHash string, days int) []map[string]interface{} {
      // SELECT DATE(created_at), COUNT(*) FROM stats
      // WHERE key_hash=$1 AND created_at >= NOW()-interval
      // GROUP BY DATE(created_at) ORDER BY date
  }

  func GetKeyLastSeen(keyHash string) string {
      // SELECT MAX(created_at) FROM stats WHERE key_hash=$1
  }

  func GetKeyErrorRate(keyHash string, days int) map[string]int {
      // SELECT code, COUNT(*) FROM errors
      // WHERE key_hash=$1 AND created_at >= NOW()-interval
      // GROUP BY code
  }

────────────────────────────────────────────────────────────────────────────
4.4 GET /admin/system/diagnostics  (~50 baris)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go:
  adminGroup.GET("/system/diagnostics", adminHandler.GetDiagnostics)

Di internal/admin/handler.go:
  func (h *Handler) GetDiagnostics(c *gin.Context) {
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()

      // Jalankan checks paralel
      redisPing := make(chan int64, 1)
      go func() {
          start := time.Now()
          cache.Ping(ctx)
          redisPing <- time.Since(start).Milliseconds()
      }()

      var memStats runtime.MemStats
      runtime.ReadMemStats(&memStats)

      redisLatency := <-redisPing

      alerts := []gin.H{}
      // Check Redis memory
      info, _ := cache.Info(ctx)
      // parse used_memory dari info string
      // kalau > 200MB (Upstash free 256MB) → tambah alert

      // Check goroutine count
      goroutines := runtime.NumGoroutine()
      if goroutines > 500 {
          alerts = append(alerts, gin.H{"level": "critical", "msg": "Goroutine count is very high — possible leak"})
      }

      c.JSON(http.StatusOK, adminResponse{Success: true, Data: gin.H{
          "uptime":           formatUptime(time.Since(h.healthHandler.StartTime())),
          "goroutines":       goroutines,
          "memory_mb":        float64(memStats.Alloc) / 1024 / 1024,
          "redis_latency_ms": redisLatency,
          "redis_ok":         redisLatency < 5000,
          "stats_db_ok":      stats.DB != nil,
          "hls_slots":        gin.H{"current": limiter.HLSDownload.Current(), "max": limiter.HLSDownload.Max()},
          "direct_slots":     gin.H{"current": limiter.DirectStream.Current(), "max": limiter.DirectStream.Max()},
          "alerts":           alerts,
      }})
  }

  // Di health/handler.go, tambah method StartTime():
  func (h *Handler) StartTime() time.Time { return h.startTime }

  // Tambah import: "runtime"

────────────────────────────────────────────────────────────────────────────
4.5 GET /admin/system/cache-stats  (~20 baris)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go:
  adminGroup.GET("/system/cache-stats", adminHandler.GetCacheStats)

Di internal/admin/handler.go:
  func (h *Handler) GetCacheStats(c *gin.Context) {
      ctx := context.Background()

      prefixes := map[string]string{
          "content:spotify":   "content:spotify:*",
          "content:tiktok":    "content:tiktok:*",
          "content:instagram": "content:instagram:*",
          "content:twitter":   "content:twitter:*",
          "content:threads":   "content:threads:*",
          "vidhub:videb":      "vidhub:videb:*",
          "vidhub:vidoy":      "vidhub:vidoy:*",
          "vidhub:vidbos":     "vidhub:vidbos:*",
          "vidhub:vidarato":   "vidhub:vidarato:*",
          "vidhub:vidnest":    "vidhub:vidnest:*",
      }

      contentCache := gin.H{}
      vidhubCache := gin.H{}

      for key, pattern := range prefixes {
          count := cache.CountKeys(ctx, pattern)
          if strings.HasPrefix(key, "content:") {
              contentCache[strings.TrimPrefix(key, "content:")] = count
          } else {
              vidhubCache[strings.TrimPrefix(key, "vidhub:")] = count
          }
      }

      shortlinks := gin.H{
          "total":  cache.CountKeys(ctx, "sl:*"),
          "active": cache.CountKeys(ctx, "sl:*"),
      }

      c.JSON(http.StatusOK, adminResponse{Success: true, Data: gin.H{
          "content_cache":      contentCache,
          "vidhub_cache":       vidhubCache,
          "shortlinks":         shortlinks,
          "app_shortlinks":     cache.CountKeys(ctx, "app:sl:*"),
          "dl_shortlinks":      cache.CountKeys(ctx, "dl:sl:*"),
          "estimated_memory_saved": "varies",
      }})
  }

────────────────────────────────────────────────────────────────────────────
4.6 GET /admin/stats/platform-breakdown  (~20 baris + 1 query SQL)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go:
  adminGroup.GET("/stats/platform-breakdown", adminHandler.GetPlatformBreakdown)

Di pkg/stats/db.go, tambah fungsi:
  func GetPlatformBreakdown() map[string]map[string]map[string]interface{} {
      rows, err := DB.Query(`
          SELECT grp, platform,
              COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE) as today,
              COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE-1
                               AND created_at < CURRENT_DATE) as yesterday
          FROM stats
          WHERE created_at >= NOW()-INTERVAL'2 days'
            AND platform IS NOT NULL AND platform != ''
          GROUP BY grp, platform
      `)
      // ... scan + build nested map + hitung trend percent
      // trend = (today - yesterday) / yesterday * 100
  }

Di internal/admin/handler.go:
  func (h *Handler) GetPlatformBreakdown(c *gin.Context) {
      data := stats.GetPlatformBreakdown()
      c.JSON(http.StatusOK, adminResponse{Success: true, Data: data})
  }

────────────────────────────────────────────────────────────────────────────
4.7 GET /admin/stats/errors/summary  (~15 baris + 2 query SQL)
────────────────────────────────────────────────────────────────────────────

Di router/admin.go:
  adminGroup.GET("/stats/errors/summary", adminHandler.GetErrorSummary)

Di pkg/stats/errors.go, tambah:
  func GetErrorSummary(hours int) map[string]interface{} {
      // Query 1: total errors + top codes
      // Query 2: errors grouped by platform
      // Query 3: errors by hour (untuk spike detection)
      // Hitung error_rate_pct = total_errors / total_requests * 100
  }

Di internal/admin/handler.go:
  func (h *Handler) GetErrorSummary(c *gin.Context) {
      hours := 24
      if hStr := c.Query("hours"); hStr != "" {
          fmt.Sscanf(hStr, "%d", &hours)
      }
      c.JSON(http.StatusOK, adminResponse{
          Success: true,
          Data:    stats.GetErrorSummary(hours),
      })
  }


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BAGIAN 5 — URUTAN IMPLEMENTASI YANG DISARANKAN
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Kalau mau mulai dari yang paling impactful:

  Hari 1 (backend ~1-2 jam):
    1. POST /admin/keys/:keyHash/reset-quota  → paling mudah, langsung berguna
    2. GET /admin/stats/quota-alerts          → deteksi masalah user sebelum komplain

  Hari 2 (backend ~2-3 jam):
    3. GET /admin/system/diagnostics          → satu endpoint untuk health overview
    4. GET /admin/system/cache-stats          → visibilitas cache, mudah implementasi

  Hari 3 (backend ~3-4 jam, butuh query SQL):
    5. GET /admin/stats/platform-breakdown    → deteksi platform drop
    6. GET /admin/stats/errors/summary        → error rate yang actionable
    7. GET /admin/keys/:keyHash/activity      → debug penggunaan per user

  HTML (setiap backend selesai, update frontend menyesuaikan):
    - Tambah section Alerts + sidebar item
    - Tambah tombol Reset Quota + Key Activity di tabel keys
    - Perbaiki loadRequests() pakai data real
    - Tambah Diagnostics + Cache Stats di Settings
    - Update loadSettings() untuk panggil semua loader baru


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  BAGIAN 6 — FILE BARU YANG PERLU DIBUAT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Tidak ada file baru yang diperlukan. Semua perubahan masuk ke file yang
sudah ada:

  internal/admin/handler.go        → tambah handler baru (ResetQuota,
                                      GetQuotaAlerts, GetDiagnostics,
                                      GetCacheStats, GetPlatformBreakdown,
                                      GetErrorSummary, GetKeyActivity)

  internal/health/handler.go       → tambah method StartTime() publik

  pkg/stats/db.go                  → tambah GetKeyDailyStats(),
                                      GetKeyLastSeen(), GetPlatformBreakdown()

  pkg/stats/errors.go              → tambah GetKeyErrorRate(), GetErrorSummary()

  router/admin.go                  → daftarkan semua route baru

  test-admin.html                  → perubahan HTML dan JS sesuai Bagian 3


━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  RINGKASAN AKHIR
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Integrasi saat ini: ~92% endpoint backend sudah di-handle frontend.
Yang belum di-handle hanya detail per app/flac dan query param filter.

Gap terbesar bukan endpoint yang hilang, melainkan OBSERVABILITY:
  - Tidak ada cara melihat quota mau habis SEBELUM user error
  - Tidak ada cara melihat platform drop dalam satu halaman
  - Error stats ada tapi tidak ada error RATE (proporsi vs total)
  - loadRequests() pakai data palsu (paling misleading)

Endpoint prioritas HIGH yang harus dibuat dulu:
  1. GET  /admin/stats/quota-alerts
  2. POST /admin/keys/:keyHash/reset-quota
  3. GET  /admin/stats/platform-breakdown

Perubahan HTML prioritas tinggi:
  1. Perbaiki loadRequests() → pakai data error real
  2. Tambah section Alerts dengan quota + platform breakdown
  3. Tambah tombol Reset Quota di key actions

================================================================================
  SELESAI
================================================================================