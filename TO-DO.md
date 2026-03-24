Priority 1 — Operasional & Stability
Structured logging ke file — sekarang semua log hanya ke stdout, hilang saat restart. Pakai lumberjack untuk rotasi log otomatis. Tanpa ini, debug production issue sangat sulit.
Gin release mode — main.go masih pakai gin.Default() tanpa gin.SetMode(gin.ReleaseMode). Di production ini memunculkan warning dan sedikit overhead debug logging yang tidak perlu.
Stats cleanup scheduler — stats.Cleanup() sudah dibuat di pkg/stats/db.go tapi tidak pernah dipanggil. Database stats akan terus membesar tanpa batas. Perlu dipanggil via goroutine di main.go, misalnya setiap hari hapus data lebih dari 90 hari.
pkg/response/response.go — WriteJSON masih pakai c.JSON() — sudah diidentifikasi sebelumnya tapi belum diperbaiki. Vidhub handler masih bisa kena \u0026 encoding issue.

Priority 2 — Developer Experience & Monitoring
API key expiry date — tambah field ExpiresAt di apikey.Data. Key trial atau key dengan batas waktu tidak bisa di-auto expire sekarang. Perlu tambah field di struct dan cek di middleware RequireAPIKey.
GET /admin/keys/:key/reset-quota — sekarang tidak ada cara reset quota counter tanpa top-up manual. Berguna untuk koreksi billing atau testing.
Cache invalidation endpoint — POST /admin/cache/clear/:service/:site untuk invalidate cache response tertentu tanpa flush seluruh Redis. Berguna kalau content di provider berubah tapi cache masih stale.
Response compression — tambah gzip middleware di router/router.go. Response IPTV channels bisa sangat besar (ratusan KB). Satu baris: r.Use(gzip.Gzip(gzip.DefaultCompression)).
GET /admin/stats?from=2026-01-01&to=2026-03-23 — sekarang stats tidak bisa di-filter by date range. SQLite sudah punya kolom created_at, tinggal tambah query parameter di GetStats.

Priority 3 — Fitur User-Facing
IPTV search endpoint — GET /iptv/channels?search=trans7. Sudah ada di backlog tapi belum diimplementasi. Data sudah in-memory, tinggal tambah filter string matching di GetChannels.
IPTV single channel — GET /iptv/channels/:id. Sering dibutuhkan kalau developer mau fetch satu channel spesifik tanpa load semua.
Webhook notifikasi quota — kirim HTTP POST ke URL yang didaftarkan ketika quota tersisa misalnya 10%. Simpan webhook_url di apikey.Data. User tidak perlu polling /auth/quota terus-menerus.
Rate limit per tier — sekarang semua key dapat limit yang sama. Tambah field tier di apikey.Data (free, pro), lalu CheckRateLimit baca limit dari tier bukan hardcode group. Sudah ada di backlog tapi belum ada struktur.
GET /admin/keys/:key/renew — perpanjang expiry date key tanpa buat key baru. Relevan kalau sudah implementasi expiry date.

Priority 4 — Security & Hardening
IP rate limiting sebagai lapis kedua — sekarang rate limit hanya per API key. Satu IP dengan banyak key bisa abuse. Tambah rate limit per IP menggunakan Redis key ratelimit:ip:{ip} dengan limit yang lebih longgar, misalnya 200 req/menit.
Request size limit — sekarang tidak ada limit ukuran request body di middleware global. Request body besar bisa abuse memory. Tambah r.Use(gin.MaxBytesMiddleware(10 * 1024 * 1024)) di router/router.go.
Admin endpoint audit log — sekarang tidak ada catatan siapa yang top-up quota, enable/disable feature, atau buat key kapan. Simpan ke SQLite atau log file terpisah.
CORS header — kalau frontend web akan akses API ini langsung dari browser, perlu tambah CORS middleware. Sekarang tidak ada.

Priority 5 — Nice to Have
POST /admin/keys/bulk-topup — top-up beberapa key sekaligus dalam satu request.
GET /health/metrics — endpoint Prometheus-compatible untuk integrasi dengan Grafana atau monitoring tools lain.
URL versioning /v1/ — sudah ada di backlog. Berguna untuk breaking changes di masa depan tanpa mematikan client lama.
Dokumentasi API publik — Postman collection atau README terpisah. Sudah ada di backlog.
GET /admin/stats/export — export stats ke CSV untuk analisis di luar aplikasi.

Ringkasan
PriorityJumlah ItemImpact1 — Operasional4Critical untuk production stability2 — Dev Experience5Memudahkan operasional harian3 — Fitur User5Meningkatkan nilai produk4 — Security4Hardening sebelum traffic besar5 — Nice to Have5Long term improvement