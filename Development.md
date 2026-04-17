# vidbot-api — Development Reference

> Dokumen ini adalah single source of truth untuk pengembangan vidbot-api.
> Update dokumen ini setiap kali ada perubahan arsitektur, endpoint baru, atau keputusan teknis.
> **Last updated:** April 2026 — audit lengkap, Redis monitoring, structured logging

---

## Build & Deploy

```bash
# build untuk Linux VPS
GOOS=linux GOARCH=amd64 go build -o vidbot-api main.go

# deploy ke VPS
# 1. upload binary via WinSCP (replace yang lama)
# 2. restart service
sudo systemctl restart vidbot-api-go

# monitoring
tail -f /home/ubuntu/vidbot-api-go/logs/out.log
tail -f /home/ubuntu/vidbot-api-go/logs/error.log
sudo systemctl status vidbot-api-go

# UNZIP leakcheck database
cd /home/ubuntu/vidbot-api-go/data/leakcheck
unzip 23Jan26.zip
rm 23Jan26.zip
```

---

## Stack

| Komponen | Teknologi |
|---|---|
| Language | Go (gin-gonic) |
| Cache / State | Redis (Upstash) — bisa 2 instance terpisah |
| Stats Tracking | SQLite (lokal, `data/stats/stats.db`) |
| App Store | SQLite per platform (lokal, `data/app/{platform}.db`) |
| Proxy Layer | Cloudflare Workers (3 worker berbeda) |
| Auth | Time-based HMAC token + API Key |
| File Conversion | CloudConvert, Convertio |
| Media Extraction | Downr + Vidown (via CF Worker) |
| HLS Processing | ffmpeg (pipe stdin→stdout, tidak ada tmp file) |
| HLS Fallback | yt-dlp (fallback kalau direct HLS gagal) |
| Logging | Go `log/slog` — structured JSON/text output |
| CDN Storage | stor.co.id (untuk App Store file hosting) |

---

## Cloudflare Workers

Ada 3 worker dengan peran berbeda — **jangan tertukar**:

| Env Var | Worker | Peran |
|---|---|---|
| `WORKER_URLS` | `de1`, `de2` | Scraping halaman (kingbokep, vidarato, videb, dll) via `proxy.Client` |
| `CONTENT_WORKER_URL` | `xcntnt1` | Content extraction (TikTok, Instagram, dll) via `contentProxyClient` |
| `DOWNLOAD_WORKER_URL` | `xvdh1` | Generate `server_1` download link — **tidak dipakai untuk scraping** |

`WORKER_URLS` bisa berisi multiple URL dipisah koma — request di-rotate secara random untuk distribusi IP.

---

## Prasyarat

Sebelum menjalankan project, pastikan tersedia:

- Go 1.21+
- Redis (Upstash atau lokal)
- Akun Cloudflare Workers (untuk proxy dan download worker)
- API key CloudConvert dan Convertio
- File binary tools di folder `tools/`: `ffmpeg`, `ffprobe`, `yt-dlp`, `N_m3u8DL-RE`, `shaka-packager`

---

## Environment Variables

Buat file `.env` di root project. Semua variabel ini wajib ada:

```env
# Redis
REDIS_URL=redis://...
CACHE_REDIS_URL=redis://...   # opsional — pisah Redis untuk response cache

# Auth
MAGIC_STRING=...
MASTER_KEY=...
STREAM_SECRET=...

# Cloudflare Workers
WORKER_URLS=https://worker1.workers.dev,https://worker2.workers.dev
WORKER_SECRET=...
CONTENT_WORKER_URL=https://content-worker.workers.dev
CONTENT_WORKER_SECRET=...

# Download Worker
DOWNLOAD_WORKER_URL=https://dl-worker.workers.dev
DOWNLOAD_WORKER_SECRET=...
WORKER_PAYLOAD_XOR_KEY=...

# App
APP_URL=http://localhost:8000
DATA_DIR=./data

# Convert Providers
CLOUDCONVERT_API_KEY=...
CONVERTIO_API_KEY=...

# Tools
TOOLS_DIR=./tools

# CDN stor.co.id (untuk App Store)
CDN_API_KEY=...
CDN_FOLDER_ID=...

# Logging (opsional, ada default)
LOG_FORMAT=json        # "json" (production) atau "text" (development)
LOG_LEVEL=info         # debug / info / warn / error

# CORS (opsional)
ALLOWED_ORIGINS=https://admin.example.com
```

---

## Logging

### Overview
Logging menggunakan Go `log/slog` (standard library sejak Go 1.21) dengan output JSON untuk production dan text untuk development. Structured logging memudahkan parsing oleh log aggregator (Datadog, Loki, dll).

### Format Output (JSON — production)
```json
{
  "time": "2026-04-17 10:30:00.123",
  "level": "INFO",
  "msg": "request completed",
  "method": "POST",
  "path": "/content/tiktok",
  "status": 200,
  "latency_ms": 847,
  "ip": "1.2.3.4",
  "request_id": "a1b2c3d4"
}
```

### Format Output (Text — development)
```
time=2026-04-17T10:30:00.123 level=INFO msg="request completed" method=POST path=/content/tiktok status=200 latency_ms=847
```

### Penggunaan di Handler Baru
```go
import (
    "log/slog"
    "vidbot-api/pkg/logger"
)

// Log sederhana
slog.Info("extract started", "url", req.URL)
slog.Warn("cache miss", "key", cacheKey)
slog.Error("extraction failed", "error", err, "url", req.URL)

// Log dengan service tag
logger.Service("vidhub", "videb").Info("extract started", "url", req.URL)
logger.Service("content", "tiktok").Error("provider failed", "provider", "downr", "error", err)

// Log dengan timing
defer logger.Timer("vidhub", "vidnest", "extract")()

// Jangan pakai log.Printf lagi — pakai slog
// ❌ log.Printf("[tiktok] extract error: %v", err)
// ✅ slog.Error("extract failed", "group", "content", "platform", "tiktok", "error", err)
```

### Mengganti log.Printf Lama
Semua `log.Printf("[x] ...")` di handler lama perlu diganti ke `slog.*`. Lihat Pending section.

---

## Menjalankan Project

```bash
# install dependencies
go mod tidy

# seed Redis (wajib dijalankan sekali sebelum pertama kali atau setelah reset Redis)
go run cmd/seed/main.go

# buat folder data (wajib ada sebelum pertama kali)
mkdir -p data/stats

# jalankan server (dev — log text)
LOG_FORMAT=text LOG_LEVEL=debug go run main.go

# jalankan server (production — log JSON)
go run main.go
```

---

## Struktur Project

```
vidbot-site-api/
├─ cmd/
│  └─ seed/
│     └─ main.go
├─ config/
│  ├─ allowed_domains.json
│  └─ config.go
├─ data/                          ← tidak masuk git (.gitignore)
│  ├─ leakcheck/
│  │  └─ leakcheck.db
│  └─ stats/
│     └─ stats.db
├─ internal/
│  ├─ admin/
│  │  ├─ handler.go               ← key, feature flag, stats management
│  │  ├─ redis_handler.go         ← ✨ NEWRedis/Upstash monitoring
│  │  └─ session_handler.go
│  ├─ auth/
│  │  ├─ handler.go
│  │  └─ service.go
│  ├─ health/
│  │  └─ handler.go
│  ├─ services/
│  │  ├─ content/
│  │  │  ├─ instagram/
│  │  │  ├─ provider/
│  │  │  │  ├─ downr/
│  │  │  │  ├─ vidown/
│  │  │  │  └─ provider.go
│  │  │  ├─ spotify/
│  │  │  ├─ threads/
│  │  │  ├─ tiktok/
│  │  │  └─ twitter/
│  │  ├─ convert/
│  │  │  ├─ audio/
│  │  │  ├─ document/
│  │  │  ├─ fonts/
│  │  │  ├─ image/
│  │  │  └─ provider/
│  │  │     ├─ cloudconvert/
│  │  │     ├─ convertio/
│  │  │     ├─ polling.go
│  │  │     └─ provider.go
│  │  ├─ iptv/
│  │  ├─ leakcheck/
│  │  ├─ app/
│  │  │  ├─ handler.go
│  │  │  └─ admin_handler.go
│  │  └─ vidhub/
│  │     ├─ kingbokeptv/
│  │     ├─ vidarato/
│  │     ├─ vidbos/
│  │     ├─ videb/
│  │     ├─ vidnest/
│  │     └─ vidoy/
│  └─ stream/
│     └─ handler.go
├─ middleware/
│  ├─ admin_rate_limit.go
│  ├─ admin_session.go
│  ├─ api_key.go
│  ├─ auth.go
│  ├─ feature.go
│  ├─ ratelimit.go
│  ├─ request_id.go
│  └─ request_logger.go           ← ✨ NEW structured HTTP access log
├─ pkg/
│  ├─ apikey/
│  ├─ appstore/
│  │  ├─ db.go
│  │  └─ shortlink.go
│  ├─ cache/
│  │  ├─ cache.go
│  │  ├─ provider_cache.go
│  │  └─ redis_info.go            ← ✨ NEW Info/PingCache/CountKeys
│  ├─ cdnstore/
│  │  ├─ client.go
│  │  └─ resolver.go
│  ├─ cloudconvert/
│  ├─ convertvalidator/
│  ├─ downloader/
│  │  ├─ cache.go
│  │  ├─ detector.go
│  │  └─ download_url.go
│  ├─ fileutil/
│  ├─ httputil/
│  ├─ iptvstore/
│  ├─ leakcheck/
│  ├─ limiter/
│  ├─ logger/
│  │  └─ logger.go                ← ✨ NEW pkg/logger — slog wrapper
│  ├─ mediaresponse/
│  ├─ proxy/
│  ├─ response/
│  ├─ shortlink/
│  ├─ stats/
│  └─ validator/
├─ router/
│  ├─ router.go
│  ├─ admin.go                    ← +GET /admin/redis/stats
│  ├─ auth.go
│  ├─ content.go
│  ├─ convert.go
│  ├─ health.go
│  ├─ iptv.go
│  ├─ leakcheck.go
│  ├─ app.go
│  └─ vidhub.go
├─ .air.toml
├─ .env
├─ .env.example
├─ go.mod
├─ go.sum
└─ main.go
```

---

## Endpoints

### Auth
| Method | Path | Keterangan |
|---|---|---|
| GET | `/auth/verify` | Verifikasi API key + dapat access token |
| GET | `/auth/quota` | Cek sisa quota API key |

### Health
| Method | Path | Keterangan |
|---|---|---|
| GET | `/health` | Cek status semua dependencies (butuh `X-Master-Key`) |

### Admin (gunakan Master Key via `X-Master-Key`)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/admin/keys` | Buat API key baru |
| DELETE | `/admin/keys/:key` | Revoke API key |
| GET | `/admin/keys` | List semua API key (`?active=true/false`) |
| POST | `/admin/keys/:key/topup` | Top-up quota |
| GET | `/admin/keys/:key/usage` | Usage detail per API key |
| POST | `/admin/keys/lookup` | Lookup API key by plain key |
| GET | `/admin/features` | Status semua feature flag |
| GET | `/admin/features/:group/enable` | Nyalakan group |
| GET | `/admin/features/:group/disable` | Matikan group |
| GET | `/admin/features/:group/:platform/enable` | Nyalakan platform |
| GET | `/admin/features/:group/:platform/disable` | Matikan platform |
| GET | `/admin/stats` | Statistik usage seluruh API |
| **GET** | **`/admin/redis/stats`** | **✨ Monitoring Redis/Upstash (memory, key count, hit rate, limits)** |

### Redis Monitoring (`GET /admin/redis/stats`)
Response:
```json
{
  "success": true,
  "checked_at": "2026-04-17T10:00:00Z",
  "connections": [
    {
      "name": "main",
      "status": "ok",
      "latency": "12ms",
      "memory_used": "18.42 MB",
      "memory_peak": "21.03 MB",
      "key_count": 0,
      "connected_clients": 1,
      "uptime": "5d 3h 22m",
      "version": "7.2.4",
      "hit_rate": "94.2% (18420 hits, 1140 misses)",
      "limits": {
        "storage_used_mb": 18.42,
        "storage_limit_mb": 256,
        "storage_pct": 7.2,
        "warning": false
      }
    },
    {
      "name": "cache",
      "status": "ok",
      ...
    }
  ],
  "key_summary": {
    "api_keys": 12,
    "rate_limits": 34,
    "content_cache": 847,
    "vidhub_cache": 213,
    "shortlinks": 1204,
    "app_shortlink": 89,
    "cdn_cache": 45,
    "features": 8,
    "providers": 9,
    "admin_session": 2
  }
}
```

Field `warning: true` pada `limits` muncul kalau memory usage > 80% dari free tier limit (256MB).

### IPTV (butuh API Key + Access Token)

#### `GET /iptv/channels`
| Query Param | Tipe | Default | Keterangan |
|---|---|---|---|
| `country` | string | — | Filter by kode negara (contoh: `ID`) |
| `category` | string | — | Filter by kategori (contoh: `news`) |
| `streams_only` | bool | `false` | Hanya channel yang punya stream aktif |
| `page` | integer | — | Nomor halaman — aktifkan pagination |
| `limit` | integer | `50` | Item per halaman, max 100 |

#### `GET /iptv/categories`
Daftar kategori. Tidak ada query params.

#### `GET /iptv/countries`
Daftar negara. Tidak ada query params.

#### `GET /iptv/playlist`
Generate file M3U untuk VLC/Tivimate. Auth via `?key=` bukan header.

### Content (butuh API Key + Access Token)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/content/spotify` | Ekstrak audio Spotify |
| POST | `/content/tiktok` | Ekstrak video/audio TikTok |
| POST | `/content/instagram` | Ekstrak video/audio Instagram |
| POST | `/content/twitter` | Ekstrak video/audio Twitter |
| POST | `/content/threads` | Ekstrak video/audio Threads |

### Vidhub (butuh API Key + Access Token)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/vidhub/videb` | Ekstrak dari Videb |
| POST | `/vidhub/vidoy` | Ekstrak dari Vidoy |
| POST | `/vidhub/vidbos` | Ekstrak dari Vidbos |
| POST | `/vidhub/vidarato` | Ekstrak dari Vidarato (HLS, master playlist 2 level) |
| POST | `/vidhub/vidnest` | Ekstrak dari Vidnest |
| POST | `/vidhub/kingbokeptv` | Ekstrak dari KingBokepTV (HLS, single level) |

### Convert (butuh API Key + Access Token)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/convert/audio` | Konversi audio via URL |
| POST | `/convert/audio/upload` | Konversi audio via upload |
| POST | `/convert/document` | Konversi dokumen via URL |
| POST | `/convert/document/upload` | Konversi dokumen via upload |
| POST | `/convert/image` | Konversi gambar via URL |
| POST | `/convert/image/upload` | Konversi gambar via upload |
| POST | `/convert/fonts` | Konversi font via URL |
| POST | `/convert/fonts/upload` | Konversi font via upload |
| GET | `/convert/status/:job_id` | Cek status job konversi |

### Stream
| Method | Path | Keterangan |
|---|---|---|
| GET | `/dl` | Proxy download stream (direct MP4 atau HLS progressive) |

### App (butuh API Key + Access Token)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/app/android` | Cari APK Android |
| GET | `/app/android/category` | Daftar kategori Android |
| GET | `/app/android/category/:category` | Browse by kategori Android |
| POST | `/app/windows` | Cari software Windows |
| GET | `/app/windows/category` | Daftar kategori Windows |
| GET | `/app/windows/category/:category` | Browse by kategori Windows |
| GET | `/app/dl?k={key}` | Redirect ke raw URL via shortlink — tidak butuh auth |

### Admin App (gunakan Master Key via `X-Master-Key`)
| Method | Path | Keterangan |
|---|---|---|
| GET | `/admin/app/:platform/list` | List semua app (`?q=keyword`, `?page=`, `?limit=`) |
| POST | `/admin/app/:platform/add` | Tambah satu entry |
| POST | `/admin/app/:platform/bulk` | Tambah banyak entry (max 200) |
| DELETE | `/admin/app/:platform/app/:slug` | Hapus app beserta semua versinya |
| DELETE | `/admin/app/:platform/version/:id` | Hapus satu versi download |
| POST | `/admin/app/:platform/cdn/invalidate` | Paksa refresh signed URL CDN |

`:platform` yang valid: `android`, `windows`

---

## Feature Flags

Feature flag memungkinkan enable/disable endpoint tanpa redeploy.
Status disimpan di Redis — efektif langsung tanpa restart server.

```
GET /admin/features/iptv/disable
GET /admin/features/content/tiktok/disable
```

### Group yang tersedia
| Group | Platform |
|---|---|
| `content` | `spotify`, `tiktok`, `instagram`, `twitter`, `threads` |
| `vidhub` | `videb`, `vidoy`, `vidbos`, `vidarato`, `vidnest`, `kingbokeptv` |
| `convert` | `audio`, `document`, `image`, `fonts` |
| `app` | `android`, `windows` |
| `iptv` | — (group level only) |
| `leakcheck` | — (group level only) |

---

## Stats Tracking

Stats disimpan di SQLite (`data/stats/stats.db`). Write async via buffered channel (kapasitas 2000).

```go
// Di setiap handler baru
stats.Platform(c, "content", "tiktok")   // endpoint dengan platform
stats.Group(c, "iptv")                    // endpoint tanpa platform
```

---

## Arsitektur Provider

### Redis Keys — Provider Priority
```
content:provider:spotify    → ["downr"]
content:provider:tiktok     → ["downr", "vidown"]
...
convert:provider:audio      → ["cloudconvert", "convertio"]
```

### Ganti provider tanpa redeploy
```bash
DEL convert:provider:audio
RPUSH convert:provider:audio convertio cloudconvert
```

---

## Redis Keys — Semua Key yang Digunakan

| Key Pattern | Tipe | Keterangan |
|---|---|---|
| `apikeys:{keyHash}` | String (JSON) | Data API key |
| `apikeys:quota:{keyHash}` | Integer | Quota terpakai |
| `apikeys:index` | Set | Index semua keyHash |
| `allowed_domains:{site}` | Set | Domain whitelist per site |
| `content:provider:{site}` | List | Urutan provider content |
| `convert:provider:{category}` | List | Urutan provider convert |
| `ratelimit:{keyHash}:{group}` | Integer (TTL 60s) | Rate limit counter per group |
| `content:{site}:{urlHash}` | String (JSON) | Cache response content |
| `vidhub:{site}:{urlHash}` | String (JSON) | Cache response vidhub |
| `feature:{group}` | String | Status on/off per group |
| `feature:{group}:{platform}` | String | Status on/off per platform |
| `sl:{key}` | String (JSON) | Shortlink payload untuk server_2 |
| `sl:idx:{cacheKey}` | String | Index shortlink → idempoten per URL |
| `app:sl:{key}` | String | App download shortlink |
| `app:sl:idx:{hash}` | String | App shortlink index |
| `cdn:app:{platform}:{slug}:{ver}` | String (JSON) | Cached signed CDN URLs |
| `admin:session:{token}` | String (JSON) | Admin session data |
| `admin:sessions:active` | Set | Index session aktif |
| `rl:admin:login:{ip}:{ua}` | Integer (TTL 60s) | Admin login rate limit |

---

## Rate Limiting

| Group | Limit |
|---|---|
| `/content/*` | 10 req/menit per API key |
| `/convert/*` | 20 req/menit per API key |
| `/vidhub/*` | 30 req/menit per API key |
| `/iptv/*` | 60 req/menit per API key |
| `/leakcheck/*` | 5 req/menit per API key |
| `/app/*` | 30 req/menit per API key |

---

## Cache TTL

| Key | TTL |
|---|---|
| `content:spotify` | 30 hari |
| `content:tiktok` | 2 jam |
| `content:instagram` | 30 menit |
| `content:threads` | 30 menit |
| `content:twitter` | 2 jam |
| `vidhub:videb` | 2 jam |
| `vidhub:vidoy` | 1 jam |
| `vidhub:vidbos` | 2 jam |
| `vidhub:vidarato` | 3 menit |
| `vidhub:vidnest` | 2 jam |
| `vidhub:kingbokeptv` | 6 jam |
| `cdn:app:*` | 6 hari (margin 1 hari dari 7-day signed URL) |
| `app:sl:*` | 5 hari |
| `sl:*` | 2 jam (content/vidhub), 30 menit (convert) |

---

## Arsitektur HLS Progressive Download

Site yang mengembalikan m3u8 (kingbokeptv, vidarato) diproses via `internal/stream/handler.go`.

### Flow
```
Client hit /dl?url={shortlink}
    ↓
Decode payload → dapat m3u8 URL + CDNOrigin
    ↓
getOrCreateSession (share session kalau URL sama)
    ↓
runDirectHLS:
  1. fetchPlaylist → handle master playlist (2 level) + segment playlist (1 level)
  2. spawn ffmpeg (stdin pipe ← .ts data, stdout pipe → mp4 chunks)
  3. download .ts segments sequential + jeda 300-800ms (anti throttle)
  4. pipe ke ffmpeg stdin → ffmpeg mux jadi mp4 fragmented
  5. baca stdout ffmpeg → append ke session chunks
    ↓ (fallback kalau direct HLS gagal)
runYTDLP → output mp4 → append ke session chunks
```

### Concurrency Limits
| Limiter | Nilai |
|---|---|
| `HLSDownload` | 3 |
| `DirectStream` | 10 |
| `cdnMaxPerHost` | 1 |

---

## Konvensi Kode

### Response
- Selalu `httputil.WriteJSONOK` atau `httputil.WriteJSON` — jangan `c.JSON()` untuk response yang mengandung URL
- Error via `response.ErrorWithCode(c, status, "CODE", "message")`
- Cache selalu disimpan **tanpa** `server_1` dan `server_2`
- `stats.Platform()` atau `stats.Group()` di baris pertama setiap handler baru
- Site HLS wajib `GenerateServer*HLSURL`, non-HLS `GenerateServer*URL`

### Logging di handler baru
```go
// ✅ Benar
slog.Error("extract failed", "group", "vidhub", "platform", "videb", "error", err, "url", req.URL)
logger.Service("vidhub", "videb").Info("cache hit", "key", cacheKey)

// ❌ Lama — jangan pakai
log.Printf("[videb] extract error: %v", err)
```

### Menambah platform content baru (misal: YouTube)

```
1. Buat folder: internal/services/content/youtube/
   → handler.go, service.go

2. pkg/downloader/cache.go → tambah TTL

3. router/content.go → tambah provider, handler, route, FeatureFlagPlatform

4. cmd/seed/main.go → tambah allowed_domains + content:provider key

5. config/allowed_domains.json → tambah domain list

6. internal/admin/handler.go → tambah ke validPlatforms["content"]

7. Di handler: stats.Platform(c, "content", "youtube") di baris pertama
```

### Menambah platform vidhub baru — Non-HLS

```
1. internal/services/vidhub/{nama}/ → handler.go, service.go
2. pkg/downloader/cache.go → TTL
3. router/vidhub.go → route
4. cmd/seed/main.go + config/allowed_domains.json
5. internal/admin/handler.go → validPlatforms["vidhub"]
6. stats.Platform(c, "vidhub", "{nama}") di baris pertama handler
7. Pakai GenerateServer1URL / GenerateServer2URL (bukan versi HLS)
```

### Menambah platform vidhub baru — HLS/M3U8

```
1. Sama seperti non-HLS + di service.go extract CDNOrigin dari m3u8 URL
2. Di handler: GenerateServer1HLSURL / GenerateServer2HLSURL
3. Saat cache hit: cdnOrigin := downloader.ExtractCDNOrigin(cached.Download.Original)
```

### Cheatsheet ringkas

| Skenario | File yang disentuh |
|---|---|
| Platform content baru | `content/{nama}/` + `cache.go` + `router/content.go` + `seed` + `allowed_domains.json` + `admin/handler.go` |
| Provider content baru | `content/provider/{nama}/` + `router/content.go` + `seed` + `router/router.go` |
| Platform vidhub baru (non-HLS) | `vidhub/{nama}/` + `cache.go` + `router/vidhub.go` + `seed` + `allowed_domains.json` + `admin/handler.go` |
| Platform vidhub baru (HLS) | sama + `GenerateServer*HLSURL` + `extractCDNOrigin` |
| Format convert baru | `service.go` + `validator.go` + `cloudconvert.go` + `convertio.go` |
| Provider convert baru | `convert/provider/{nama}/` + `router/convert.go` |
| Platform app baru | `db.go` + handler baru + `router/app.go` + `admin/handler.go` |

---

## Format Konversi yang Didukung

### Audio
`mp3, wav, flac, aac, ogg, m4a, opus, wma, amr, ac3`

### Document
`pdf, docx, xlsx, pptx, txt, html, odt, rtf, md, xls, csv, ppt, wps, dotx, docm, doc`

### Image
`jpg, jpeg, png, webp, gif, avif, bmp, ico, jfif, tiff, psd, raf, mrw, heic, heif, eps, svg, raw`

### Fonts
`ttf, otf, woff, woff2, eot`

---

## App Store

### Overview
Menyimpan APK/software modded. SQLite per platform. Raw URL di-mask via Redis shortlink.
File fisik di-host di CDN stor.co.id. Signed URL di-cache 6 hari.

### File Naming Convention di CDN
```
{app-slug}_{version}_{variant}.apk
contoh: spotiflac_4.3.1_arm64-v8a.apk
        spotiflac_4.3.1_universal.apk
        idm_6.42_setup.exe
```

### CDN Cache Key
```
cdn:app:{platform}:{appSlug}:{version}
cdn:app:android:spotiflac:4.3.1
cdn:app:android:spotiflac:all        ← version kosong = semua versi
```

---

## Audit Temuan — April 2026

### Bug yang sudah diperbaiki ✅
| # | Bug | File | Status |
|---|---|---|---|
| 1 | `/convert/image/upload` validasi pakai `Audio` bukan `Image` | `convert/image/handler.go` | ✅ Fixed |
| 2 | `content:threads` tidak ada di `cacheTTL` | `pkg/downloader/cache.go` | ✅ Fixed |
| 3 | `iptvstore.startRefresh()` tidak dipanggil | `pkg/iptvstore/store.go` | ✅ Fixed |
| 4 | `deriveOrigin` dead code | `internal/stream/handler.go` | ✅ Fixed |
| 5 | Limiter `HLSDownload` di-release terlalu cepat | `internal/stream/handler.go` | ✅ Fixed |
| 6 | Log ffmpeg/yt-dlp terlalu verbose | `internal/stream/handler.go` | ✅ Fixed |
| 7 | Master playlist vidarato tidak di-resolve | `internal/stream/handler.go` | ✅ Fixed |
| 8 | HLS session memory leak | `internal/stream/handler.go` | ✅ Fixed |
| 9 | `appstore` — `SetMaxOpenConns(1)` untuk read | `pkg/appstore/db.go` | ✅ Fixed (pisah writeDB/readDB) |
| 10 | `appstore` — tidak ada `PRAGMA foreign_keys=ON` | `pkg/appstore/db.go` | ✅ Fixed |
| 11 | `appstore` — `Upsert` tidak dalam transaksi | `pkg/appstore/db.go` | ✅ Fixed |
| 12 | `appstore` — `toSlug` collision | `pkg/appstore/db.go` | ✅ Fixed (fallback angka) |
| 13 | `appstore` — trigger `apps_au` missing | `pkg/appstore/db.go` | ✅ Fixed |
| 14 | `appstore` — DDL multiple statement dalam satu Exec | `pkg/appstore/db.go` | ✅ Fixed |
| 15 | `appstore` — shortlink `hashKey` pakai XOR | `pkg/appstore/shortlink.go` | ✅ Fixed (sha256) |
| 16 | `appstore` — shortlink TTL 72 jam | `pkg/appstore/shortlink.go` | ✅ Fixed (5 hari) |
| 17 | `appstore` — `SearchAll` tanpa LIMIT | `pkg/appstore/db.go` | ✅ Fixed (pagination) |
| 18 | `appstore` — FTS query tanpa LIMIT | `pkg/appstore/db.go` | ✅ Fixed (LIMIT 50) |
| 19 | `appstore` — tidak ada `mmap_size` PRAGMA | `pkg/appstore/db.go` | ✅ Fixed |
| 20 | `appstore` — `normPlatform` silent fallback | `admin_handler.go` | ✅ Fixed (return 400) |
| 21 | `appstore` — route konflik `/:platform/:slug` | `router/app.go` | ✅ Fixed (prefix `/app/`) |
| 22 | `appstore` — tidak ada `rows.Err()` check | `pkg/appstore/db.go` | ✅ Fixed |
| 23 | `appstore` — N+1 query `getDownloads` | `pkg/appstore/db.go` | ✅ Fixed (`batchGetVersions`) |
| 24 | gin.Default() dipakai di production | `main.go` | ✅ Fixed (pakai gin.New() + custom middleware) |

### Bug yang masih ada 🔴🟡
| # | Bug | File | Priority |
|---|---|---|---|
| B1 | `response.WriteJSON` di vidbos, videb, vidoy, vidarato handler — bisa corrupt URL | `vidhub/*` handlers | 🟡 Medium |
| B2 | Timing log verbose di `vidnest/service.go` | `vidnest/service.go` | 🟡 Low |
| B3 | `log.Printf` tersebar di semua handler — belum diganti ke `slog` | semua handler | 🟡 Medium |
| B4 | Goroutine secondary di content service tanpa context cancellation | `content/*/service.go` | 🟡 Low |
| B5 | Fix ID dan Duration kosong di response TikTok | `tiktok/handler.go` | 🟡 Medium |
| B6 | CF Worker: Referer header untuk Convertio URLs (server_1 masih 403) | CF Worker config | 🔴 High |
| B7 | `stats.db` PRAGMA multiple statement dalam satu Exec | `pkg/stats/db.go` | 🟡 Medium |

### Security Observations
| # | Temuan | Severity | Notes |
|---|---|---|---|
| S1 | `MASTER_KEY` dikirim via header `X-Master-Key` di setiap request admin | Low | Aman kalau HTTPS, pertimbangkan session-based untuk admin UI |
| S2 | Rate limit login admin hanya per IP+UA — bisa bypass dengan ganti UA | Low | Cukup untuk proteksi basic |
| S3 | API key quota check pakai Lua script — race condition minimal tapi masih ada window | Low | Acceptable untuk use case ini |
| S4 | `PAYLOAD_ENCRYPT_KEY` dan `PAYLOAD_HMAC_KEY` di `.env` — jangan commit ke git | Medium | Pastikan `.gitignore` include `.env` |

### Performance Observations
| # | Temuan | Impact | Rekomendasi |
|---|---|---|---|
| P1 | Provider cache sync setiap 5 menit — stale selama 5 menit setelah Redis update | Low | Tambah manual refresh endpoint kalau perlu |
| P2 | `CountKeys` di Redis monitoring pakai SCAN — aman tapi O(n) | Low | Cache hasil monitoring 60 detik |
| P3 | Session cleanup admin pakai probabilistic (1/10 request) | Low | Cukup untuk volume rendah |
| P4 | `iptv` store load di startup — blocking kalau network lambat | Medium | Buat async dengan channel + timeout |
| P5 | `convertvalidator` HEAD request per conversion — tambah 3-10 detik latency | Medium | Cache HEAD result per URL selama 5 menit |

---

## Pending / Backlog

### High Priority 🔴
- [ ] Fix `response.WriteJSON` → `httputil.WriteJSONOK` di vidbos, videb, vidoy, vidarato handler (B1)
- [ ] Fix CF Worker Referer header untuk Convertio server_1 (B6)
- [ ] Ganti semua `log.Printf` ke `slog.*` di semua handler (B3) — lihat contoh di bagian Logging

### Medium Priority 🟡
- [ ] Fix ID dan Duration kosong di response TikTok (B5)
- [ ] Cache hasil `convertvalidator` HEAD per URL (P5)
- [ ] Hapus timing log verbose di `vidnest/service.go` (B2)
- [ ] Pisah PRAGMA di `pkg/stats/db.go` per statement (B7)
- [ ] Cache hasil monitoring Redis selama 60 detik (P3)

### Low Priority 🟢
- [ ] Context cancellation di goroutine secondary content service (B4)
- [ ] Async IPTV store load di startup (P4)
- [ ] Cache hasil convert untuk hemat credits CloudConvert/Convertio
- [ ] URL versioning `/v1/`
- [ ] Dokumentasi API publik (Postman collection)
- [ ] Tier sistem (free, pro, enterprise) untuk rate limit + quota berbeda
- [ ] Structured logging ke file (rotate harian) — saat ini hanya stdout
- [ ] Cleanup stats scheduler (hapus data > 90 hari)
- [ ] Skip cache sepenuhnya untuk vidarato

### Sudah Selesai ✅
- [x] Health check endpoint (`GET /health`)
- [x] Provider priority pindah ke memory
- [x] Konsolidasi `writeJSONUnescaped` ke `pkg/httputil`
- [x] Konsolidasi `sanitizeFilename` ke `pkg/fileutil`
- [x] Pecah `router/router.go` ke sub-router per grup
- [x] Graceful shutdown
- [x] Feature flag per group dan platform
- [x] Stats tracking per platform via SQLite
- [x] IPTV playlist endpoint
- [x] gin.ReleaseMode di production
- [x] Async stats write via buffered channel
- [x] HLS progressive download dengan ffmpeg pipe
- [x] Master playlist auto-resolve (vidarato 2-level)
- [x] CDNOrigin di payload HLS
- [x] Session sharing untuk HLS
- [x] freeChunks() setelah semua reader done
- [x] Natural delay antar segment HLS
- [x] CDN concurrent limiter per host
- [x] App Store module lengkap dengan CDN integration
- [x] Admin session management (login/logout/me)
- [x] Browse by category untuk App Store
- [x] CDN cache invalidation endpoint
- [x] **Redis/Upstash monitoring** (`GET /admin/redis/stats`) ✨
- [x] **Structured logging** via `log/slog` + `pkg/logger` ✨
- [x] **Request logging middleware** yang terstruktur ✨
- [x] gin.New() + custom middleware (hapus gin.Default()) ✨

---

## Keputusan Teknis

| Keputusan | Alasan |
|---|---|
| Provider pattern dengan Redis priority | Ganti provider tanpa redeploy |
| `from` wajib di convert | Cegah hit provider untuk kombinasi format yang tidak support |
| Cache tanpa server_1/server_2 | URL download berisi HMAC yang time-based |
| `httputil.WriteJSONOK` | Mencegah `\u0026` di URL |
| Sub-router per grup | `router/router.go` tidak perlu disentuh saat tambah platform |
| Rate limit per group via Redis | Bisa diubah tanpa redeploy |
| Stats tracking via SQLite | Mengurangi hit Redis, data permanen |
| Feature flag via Redis | Real-time toggle tanpa restart |
| HLS pipe via ffmpeg stdin/stdout | Tidak ada tmp file di disk |
| Session sharing untuk HLS | Tidak spawn 2 proses ffmpeg untuk URL sama |
| CDNOrigin di payload HLS terenkripsi | Tidak perlu mapping static di code |
| `GenerateServer*HLSURL` terpisah | Zero breaking change untuk site non-HLS |
| DB SQLite terpisah per platform | Isolasi data, mudah backup |
| FTS5 untuk search app | Performa jauh lebih baik |
| `log/slog` untuk logging | Standard library sejak Go 1.21, structured output, zero dependency |
| `gin.New()` bukan `gin.Default()` | Kontrol penuh atas middleware; default logger gin tidak structured |
| Redis SCAN bukan KEYS untuk monitoring | KEYS memblok Redis saat data besar; SCAN safe untuk production |
| Upstash free tier warning di 80% | Buffer 20% untuk tidak kaget saat mendekati limit 256MB |