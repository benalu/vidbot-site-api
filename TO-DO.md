# Vidbot API — Audit, Review & Roadmap

> Tanggal audit: April 2026  
> Reviewer: Claude (Anthropic)  
> Scope: Full codebase review untuk standar SaaS production

---

## 1. Ringkasan Eksekutif

Vidbot API adalah API multi-layanan yang mencakup content extraction (TikTok, Instagram, Twitter, Spotify, Threads), video hosting extraction (vidhub), file conversion (audio, dokumen, image, font), IPTV, app store, dan leak check. Arsitektur secara umum sudah cukup baik untuk tahap awal, namun ada beberapa gap signifikan sebelum bisa dianggap production-ready SaaS.

**Skor kesehatan per area:**

| Area | Status | Catatan |
|------|--------|---------|
| Auth & Security | ⚠️ Sedang | Tidak ada admin session, token HMAC 5 menit terlalu pendek untuk beberapa use case |
| API Design | ✅ Baik | Konsisten, RESTful, error response sudah terstandar |
| Caching | ✅ Baik | Redis + dual client, shortlink idempoten |
| Rate Limiting | ⚠️ Sedang | Hanya per-group, belum ada IP-based atau burst protection |
| Observability | ⚠️ Sedang | SQLite stats ada, tapi tidak ada structured logging, tracing, atau alerting |
| Error Handling | ⚠️ Sedang | Banyak `json.Unmarshal` tanpa cek error dibuang, silent failures |
| Testing | ❌ Tidak Ada | Zero test coverage yang terlihat di codebase |
| Documentation | ❌ Tidak Ada | Tidak ada OpenAPI/Swagger spec |
| Admin Panel | ❌ Tidak Ada | Semua admin via raw HTTP, tidak ada UI, tidak ada session auth |
| CORS | ⚠️ Hardcoded | `localhost:5501` hardcoded di main.go |

---

## 2. Fitur Baru: Admin Session Auth (Endpoint Login)

### Latar Belakang

Saat ini admin diautentikasi via `X-Master-Key` header di setiap request. Ini aman untuk CLI/curl, tapi tidak cocok untuk integrasi frontend karena:
- Master key terekspos di setiap request
- Tidak ada session TTL atau revocation
- Tidak ada audit trail siapa yang login kapan

### Desain yang Direkomendasikan

#### File Baru yang Perlu Dibuat

**`internal/admin/session.go`**
```go
package admin

// AdminSession — struktur session admin
type AdminSession struct {
    SessionID string    `json:"session_id"`
    CreatedAt time.Time `json:"created_at"`
    ExpiresAt time.Time `json:"expires_at"`
    IPAddress string    `json:"ip_address"`
}
```

**`internal/admin/session_handler.go`**

Tambahkan 3 endpoint baru ke handler yang sudah ada:

```
POST /admin/auth/login     → Validasi master key, return session token (JWT atau random token + Redis)
POST /admin/auth/logout    → Revoke session token
GET  /admin/auth/me        → Cek session aktif
```

**Implementasi Login:**
```go
func (h *Handler) Login(c *gin.Context) {
    var req struct {
        MasterKey string `json:"master_key"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        // return 400
    }
    if req.MasterKey != h.masterKey {
        // return 401, tambahkan delay 500ms untuk anti-brute-force
        time.Sleep(500 * time.Millisecond)
        return
    }
    
    // Generate session token (32 bytes random, hex encoded)
    raw := make([]byte, 32)
    rand.Read(raw)
    sessionToken := hex.EncodeToString(raw)
    
    // Simpan di Redis: "admin:session:{token}" → JSON metadata
    // TTL: 8 jam (bisa konfigurasikan via env ADMIN_SESSION_TTL)
    sessionData := AdminSession{
        SessionID: sessionToken,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(8 * time.Hour),
        IPAddress: c.ClientIP(),
    }
    
    // Return: { success: true, session_token: "...", expires_at: "..." }
}
```

**`middleware/admin_session.go`** — middleware baru untuk proteksi route admin frontend

```go
func RequireAdminSession() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("X-Admin-Session")
        if token == "" {
            token = c.Query("session") // fallback untuk query param
        }
        // Lookup di Redis, validasi TTL, set context
    }
}
```

**Perubahan di `router/admin.go`:**
```go
// Tambahkan route baru:
adminGroup.POST("/auth/login", adminHandler.Login)
adminGroup.POST("/auth/logout", middleware.RequireAdminSession(), adminHandler.Logout)
adminGroup.GET("/auth/me", middleware.RequireAdminSession(), adminHandler.Me)

// Route admin yang sudah ada bisa tetap pakai X-Master-Key
// ATAU migrasi ke RequireAdminSession() secara bertahap
```

**Perubahan di `config/config.go`:**
```go
// Tambahkan field:
AdminSessionTTL time.Duration // default 8 jam, dari env ADMIN_SESSION_TTL
```

**Perubahan di `.env.example`:**
```
ADMIN_SESSION_TTL=8h
```

#### Redis Key Schema untuk Sessions
```
admin:session:{token}     → JSON AdminSession, TTL 8 jam
admin:sessions:active     → SET of active session IDs (untuk audit)
```

---

## 3. Critical Issues (Harus Diperbaiki Sebelum Production)

### 3.1 CORS Hardcoded — `main.go`

**Masalah:** Origin `http://localhost:5501` hardcoded.

**File:** `main.go`, baris CORS middleware

**Ganti dengan:**
```go
// Tambah ke config:
AllowedOrigins []string // dari env: ALLOWED_ORIGINS=https://app.domain.com,https://admin.domain.com

// Di main.go:
allowedOrigins := strings.Split(cfg.AllowedOrigins, ",")
// Gunakan map untuk O(1) lookup
```

### 3.2 Silent Error Discards

**Masalah:** `json.Unmarshal([]byte(raw), &data)` tanpa menggunakan return error di banyak tempat.

**File:** `internal/admin/handler.go` — ada ~8 tempat, `middleware/api_key.go`

**Pattern yang salah:**
```go
json.Unmarshal([]byte(raw), &data) // error dibuang
```

**Ganti dengan:**
```go
if err := json.Unmarshal([]byte(raw), &data); err != nil {
    log.Printf("[admin] unmarshal key data: %v", err)
    // handle error
}
```

### 3.3 Tidak Ada Request ID / Tracing

**Masalah:** Tidak ada correlation ID di setiap request, membuat debugging di production sangat sulit.

**Buat file baru: `middleware/request_id.go`**
```go
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader("X-Request-ID")
        if id == "" {
            id = generateRequestID() // uuid atau ulid
        }
        c.Set("request_id", id)
        c.Header("X-Request-ID", id)
        c.Next()
    }
}
```

Tambahkan middleware ini pertama kali di `main.go` sebelum router setup.

### 3.4 Quota Check Race Condition — `middleware/api_key.go`

**Masalah:** Pattern `Incr → cek → Decr` tidak atomic. Dalam kondisi concurrent tinggi, dua request bisa lolos quota secara bersamaan.

**Solusi:** Gunakan Lua script di Redis untuk atomic check-and-increment:
```lua
local current = redis.call('GET', KEYS[1])
if current and tonumber(current) >= tonumber(ARGV[1]) then
    return 0
end
return redis.call('INCR', KEYS[1])
```

**Tambahkan ke `pkg/cache/cache.go`:**
```go
func AtomicQuotaCheck(ctx context.Context, key string, limit int) (bool, error) {
    // Jalankan Lua script
}
```

### 3.5 Shutdown Timeout Terlalu Panjang — `main.go`

**Masalah:** `10 * time.Minute` untuk shutdown context sangat lama, akan memperlambat deployment.

**Ganti:** Pisahkan timeout untuk HLS session drain (maks 2 menit) dan HTTP server shutdown (30 detik).

---

## 4. Security Issues

### 4.1 Master Key Exposed di URL Params

**File:** `router/admin.go` — beberapa endpoint menggunakan GET method untuk operasi yang seharusnya POST/PUT (enable/disable feature). GET request bisa ter-log di proxy/CDN termasuk header.

**Ganti:**
```go
// SEBELUM:
adminGroup.GET("/features/:group/enable", ...)
adminGroup.GET("/features/:group/disable", ...)

// SESUDAH:
adminGroup.PUT("/features/:group", ...)  // body: {"status": "on"/"off"}
```

### 4.2 Tidak Ada Brute Force Protection di API Key Validation

**File:** `middleware/api_key.go`

**Tambah:** Rate limit per IP untuk request yang gagal auth (misal: max 20 kali per menit per IP).

**Buat file baru: `middleware/ip_ratelimit.go`** menggunakan Redis dengan key `auth:fail:{ip}`.

### 4.3 `X-Master-Key` Tidak Punya Expiry atau Rotation

**Masalah:** Master key adalah static secret tanpa mekanisme rotasi.

**Rekomendasi jangka panjang:** Setelah admin session selesai, semua operasi admin migrasikan ke session token. Master key hanya untuk bootstrap (create first session saja).

### 4.4 Worker XOR Key Lemah

**File:** `pkg/downloader/download_url.go`

XOR dengan key yang pendek dan posisi byte adalah enkripsi yang sangat lemah. Worker payload bisa di-reverse engineer dengan mudah.

**Rekomendasi:** Ganti dengan AES-GCM yang sama seperti Server 2 payload, atau setidaknya gunakan HMAC untuk integrity check.

---

## 5. Architecture Issues

### 5.1 Stats di SQLite — Bottleneck Saat Scale

**File:** `pkg/stats/db.go`

`SetMaxOpenConns(1)` untuk SQLite write akan jadi bottleneck serius saat traffic tinggi. Setiap stats track adalah synchronous INSERT meskipun sudah ada channel buffer.

**Opsi solusi:**
- Batch insert: kumpulkan 100 events lalu insert sekaligus
- Migrasi ke Redis sorted sets / Redis Streams untuk real-time stats
- Atau gunakan service terpisah (ClickHouse/TimescaleDB) untuk analytics

### 5.2 HLS Session Management — Memory Leak Potential

**File:** `internal/stream/handler.go`

`hlsSessions` map tidak punya hard limit jumlah sessions. Jika banyak client mulai download lalu disconnect sebelum 2 menit, sessions akan menumpuk di memory sampai `sessionTTL` (10 menit).

**Tambahkan:** Max sessions limit (misal 50) di `getOrCreateSession`, return 503 jika sudah penuh.

### 5.3 Tidak Ada Health Check per Dependency

**File:** `internal/health/handler.go`

Health check sudah ada tapi tidak digunakan untuk circuit breaker. Jika CloudConvert `down`, request tetap diproses dan baru gagal saat submit.

**Rekomendasi:** Implementasi simple circuit breaker per provider.

### 5.4 Provider Order Cache — Stale Data

**File:** `pkg/cache/provider_cache.go`

Provider cache refresh setiap 5 menit. Jika Redis down saat startup, `data` map kosong dan `GetProviderOrder` return empty slice, fallback ke urutan default. Ini aman tapi perlu di-log.

---

## 6. Code Quality Issues

### 6.1 Duplikasi Kode Ekstrem di Content Handlers

File `tiktok/handler.go`, `instagram/handler.go`, `twitter/handler.go` memiliki struktur yang **identik 90%**. Ini melanggar DRY principle dan setiap bug fix harus dilakukan di 5 tempat.

**Refactor yang disarankan:** Buat `content/base_handler.go` dengan generic handler:
```go
type BaseContentHandler[TResponse any] struct {
    extract      func(url string) (*ExtractionResult, error)
    buildResponse func(result *ExtractionResult, urls DownloadURLs) TResponse
    // ...
}
```

### 6.2 Duplikasi di Convert Handlers

Sama seperti content handlers, `audio/handler.go`, `document/handler.go`, `image/handler.go`, `fonts/handler.go` memiliki pola identik.

### 6.3 `strings.Title` Deprecated

**File:** `internal/services/iptv/handler.go`

```go
groupTitle = strings.Title(ch.Categories[0]) // deprecated sejak Go 1.18
```

**Ganti dengan:** `golang.org/x/text/cases` package.

### 6.4 Magic Numbers Tersebar

**Contoh:**
- `rand.Intn(700)+500` di `downr.go` (jeda 500-1200ms)
- `bitrateKbps = 767` di `vidnest/service.go`
- `8 * time.Hour` di session (belum ada tapi saat dibuat)

**Pindahkan ke konstanta bernama** di masing-masing file.

### 6.5 `min()` Function Redefined

**File:** `internal/services/vidhub/vidbos/service.go` dan `pkg/proxy/proxy.go` keduanya define `func min()`. Di Go 1.21+ sudah ada builtin `min()`.

**Hapus** kedua definisi tersebut jika menggunakan Go 1.21+.

---

## 7. Missing SaaS Features

### 7.1 Tidak Ada Webhook / Callback

Untuk conversion yang lama (dokumen besar), client harus polling `/convert/status/:job_id`. SaaS yang baik menyediakan webhook saat job selesai.

**Tambahkan:**
- Field `webhook_url` di convert request
- Background goroutine yang POST ke webhook saat job finish

### 7.2 Tidak Ada Email Notification

Tidak ada sistem notifikasi saat quota hampir habis, API key expired, dsb.

### 7.3 Tidak Ada Usage Dashboard untuk User

User hanya bisa cek quota via `GET /auth/quota`. Tidak ada breakdown per endpoint, trend harian, dsb.

### 7.4 Tidak Ada Versioning API

Semua endpoint di `/content/*`, `/vidhub/*` tanpa prefix versi. Jika perlu breaking change, tidak ada path migrasi.

**Rekomendasi:** Prefix `/v1/content/*` untuk semua endpoint baru.

### 7.5 Tidak Ada Pagination di Beberapa Endpoint

- `GET /admin/keys` — tidak ada pagination, jika ada ribuan key ini bisa OOM
- `GET /iptv/channels` — sudah ada pagination opsional, bagus

**Tambahkan mandatory pagination** di `/admin/keys` dengan default limit 50.

### 7.6 Tidak Ada Soft Delete untuk API Keys

`RevokeKey` set `active=false` tapi data tetap ada. Tidak ada cara untuk benar-benar menghapus key atau melihat history revocation.

---

## 8. TODO List Prioritas

### 🔴 CRITICAL (Sprint 1 — Sebelum Production)

- [ ] **Buat `internal/admin/session_handler.go`** — Implementasi POST `/admin/auth/login`, `/admin/auth/logout`, GET `/admin/auth/me`
- [ ] **Buat `middleware/admin_session.go`** — Session validation middleware
- [ ] **Fix CORS hardcode di `main.go`** — Pindah ke env var `ALLOWED_ORIGINS`
- [ ] **Fix silent error discards** di `internal/admin/handler.go` (8 tempat) dan `middleware/api_key.go`
- [ ] **Buat `middleware/request_id.go`** — Correlation ID untuk semua request
- [ ] **Tambahkan `ADMIN_SESSION_TTL` ke `config/config.go` dan `.env.example`**
- [ ] **Fix quota race condition** di `middleware/api_key.go` — Gunakan atomic Lua script
- [ ] **Tambah pagination** di `GET /admin/keys`

### 🟡 HIGH (Sprint 2)

- [ ] **Refactor content handlers** — Extract base handler untuk menghilangkan duplikasi di tiktok/instagram/twitter/threads/spotify
- [ ] **Refactor convert handlers** — Sama seperti content handlers
- [ ] **Buat `middleware/ip_ratelimit.go`** — Brute force protection untuk auth endpoints
- [ ] **Fix `strings.Title` deprecated** di `internal/services/iptv/handler.go`
- [ ] **Tambah max sessions limit** di `internal/stream/handler.go`
- [ ] **Ganti GET method dengan PUT/PATCH** untuk feature flag toggle di `router/admin.go` dan `internal/admin/handler.go`
- [ ] **Hapus duplikasi `min()` function** di `vidbos/service.go` dan `pkg/proxy/proxy.go`
- [ ] **Batch insert untuk stats** di `pkg/stats/db.go` — Kumpulkan 100 events sebelum flush

### 🟢 MEDIUM (Sprint 3)

- [ ] **Implementasi webhook** untuk convert jobs — Tambah `webhook_url` field di request
- [ ] **API versioning** — Prefix `/v1/` untuk semua endpoint
- [ ] **Buat `openapi.yaml`** — Dokumentasi API lengkap (bisa generate dari code)
- [ ] **Unit tests** untuk `pkg/cache`, `pkg/downloader`, `internal/auth/service.go`
- [ ] **Integration tests** untuk middleware stack
- [ ] **Worker XOR payload** — Ganti dengan HMAC-signed payload
- [ ] **Structured logging** — Ganti `log.Printf` dengan `slog` (Go 1.21+) atau `zerolog`
- [ ] **Circuit breaker** untuk convert providers (CloudConvert, Convertio)

### 🔵 LOW (Sprint 4+)

- [ ] **Migrasi stats** dari SQLite ke Redis Streams atau TimescaleDB
- [ ] **Usage dashboard endpoint** untuk user (breakdown per endpoint, trend)
- [ ] **Email notification** untuk quota warning (80%, 100%)
- [ ] **Soft delete + history** untuk API key revocation
- [ ] **Dockerfile + docker-compose.yml** — Container setup untuk deployment
- [ ] **GitHub Actions CI/CD** — Lint, test, build pipeline
- [ ] **Metric export** — Prometheus endpoint untuk monitoring

---

## 9. Contoh Implementasi Lengkap: Admin Login

Berikut adalah blueprint lengkap file yang perlu dibuat:

### File 1: `internal/admin/session_handler.go` (BUAT BARU)

```go
package admin

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "vidbot-api/pkg/cache"

    "github.com/gin-gonic/gin"
)

const (
    adminSessionPrefix = "admin:session:"
    adminSessionIndex  = "admin:sessions:active"
    defaultSessionTTL  = 8 * time.Hour
)

type AdminSessionData struct {
    SessionID string `json:"session_id"`
    CreatedAt string `json:"created_at"`
    ExpiresAt string `json:"expires_at"`
    IPAddress string `json:"ip_address"`
    UserAgent string `json:"user_agent"`
}

func (h *Handler) Login(c *gin.Context) {
    var req struct {
        MasterKey string `json:"master_key"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false, "code": "BAD_REQUEST", "message": "master_key is required",
        })
        return
    }

    // Constant-time compare + delay untuk anti-brute-force
    if req.MasterKey != h.masterKey {
        time.Sleep(500 * time.Millisecond)
        c.JSON(http.StatusUnauthorized, gin.H{
            "success": false, "code": "UNAUTHORIZED", "message": "Invalid credentials",
        })
        return
    }

    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate session"})
        return
    }
    token := hex.EncodeToString(raw)

    ttl := defaultSessionTTL // bisa dari config nanti
    sessionData := AdminSessionData{
        SessionID: token,
        CreatedAt: time.Now().UTC().Format(time.RFC3339),
        ExpiresAt: time.Now().Add(ttl).UTC().Format(time.RFC3339),
        IPAddress: c.ClientIP(),
        UserAgent: c.GetHeader("User-Agent"),
    }

    data, _ := json.Marshal(sessionData)
    ctx := context.Background()
    if err := cache.Set(ctx, adminSessionPrefix+token, string(data), ttl); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to save session"})
        return
    }
    cache.SAdd(ctx, adminSessionIndex, token)

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "session_token": token,
        "expires_at":    sessionData.ExpiresAt,
    })
}

func (h *Handler) Logout(c *gin.Context) {
    token := c.GetHeader("X-Admin-Session")
    ctx := context.Background()
    cache.Del(ctx, adminSessionPrefix+token)
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out"})
}

func (h *Handler) Me(c *gin.Context) {
    token := c.GetHeader("X-Admin-Session")
    ctx := context.Background()
    raw, err := cache.Get(ctx, adminSessionPrefix+token)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"success": false, "code": "UNAUTHORIZED", "message": "Session not found"})
        return
    }
    var data AdminSessionData
    json.Unmarshal([]byte(raw), &data)
    c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ValidateAdminSession — dipakai oleh middleware
func ValidateAdminSession(token string) (*AdminSessionData, error) {
    ctx := context.Background()
    raw, err := cache.Get(ctx, adminSessionPrefix+token)
    if err != nil {
        return nil, fmt.Errorf("session not found or expired")
    }
    var data AdminSessionData
    if err := json.Unmarshal([]byte(raw), &data); err != nil {
        return nil, fmt.Errorf("invalid session data")
    }
    return &data, nil
}
```

### File 2: `middleware/admin_session.go` (BUAT BARU)

```go
package middleware

import (
    "net/http"
    "vidbot-api/internal/admin"
    "github.com/gin-gonic/gin"
)

func RequireAdminSession() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("X-Admin-Session")
        if token == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "code":    "UNAUTHORIZED",
                "message": "Admin session required. Please login via POST /admin/auth/login",
            })
            return
        }
        sessionData, err := admin.ValidateAdminSession(token)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "code":    "SESSION_EXPIRED",
                "message": "Session expired or invalid. Please login again.",
            })
            return
        }
        c.Set("admin_session", sessionData)
        c.Next()
    }
}
```

### Perubahan di File yang Sudah Ada

**`router/admin.go`** — Tambahkan 3 baris route:
```go
// Tambahkan di dalam setupAdmin, SEBELUM adminGroup := r.Group("/admin"):
r.POST("/admin/auth/login", adminHandler.Login)

// Tambahkan di dalam adminGroup:
adminGroup.POST("/auth/logout", middleware.RequireAdminSession(), adminHandler.Logout)
adminGroup.GET("/auth/me", middleware.RequireAdminSession(), adminHandler.Me)
```

**`.env.example`** — Tambahkan baris:
```
ADMIN_SESSION_TTL=8h
```

---

## 10. Catatan Arsitektur Jangka Panjang

### Multitenancy
Saat ini quota dan stats tracking sudah per API key, ini fondasi yang bagus. Untuk SaaS sejati, pertimbangkan menambahkan konsep `organization` atau `workspace` di atas API key.

### Service Separation
Content extraction (vidhub, social media) dan file conversion adalah dua concern yang berbeda. Pertimbangkan memisahkan menjadi microservice terpisah jika traffic per layanan sudah berbeda jauh.

### CDN untuk Download
Saat ini `server_2` langsung relay dari server Go. Untuk skala besar, pertimbangkan signed URL langsung ke CDN (CloudFront, BunnyCDN) tanpa lewat server.

### Database untuk API Keys
Menyimpan API key di Redis sudah cukup untuk skala kecil-menengah, tapi Redis bukan database. Pertimbangkan backup otomatis atau migrasi ke PostgreSQL untuk data yang critical seperti billing dan quota.

---

*Dokumen ini dibuat berdasarkan analisis static codebase. Beberapa isu mungkin sudah ditangani di konfigurasi deployment yang tidak terlihat di sini.*