# vidbot-api — Development Reference

> Dokumen ini adalah single source of truth untuk pengembangan vidbot-api.
> Update dokumen ini setiap kali ada perubahan arsitektur, endpoint baru, atau keputusan teknis.

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
```

---

## Stack

| Komponen | Teknologi |
|---|---|
| Language | Go (gin-gonic) |
| Cache / State | Redis (Upstash) |
| Stats Tracking | SQLite (lokal, `data/stats/stats.db`) |
| Proxy Layer | Cloudflare Workers |
| Auth | Time-based HMAC token + API Key |
| File Conversion | CloudConvert, Convertio |
| Media Extraction | Downr + Vidown (via CF Worker) |

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

# Convert Providers
CLOUDCONVERT_API_KEY=...
CONVERTIO_API_KEY=...

# Tools
TOOLS_DIR=./tools
```

---

## Menjalankan Project

```bash
# install dependencies
go mod tidy

# seed Redis (wajib dijalankan sekali sebelum pertama kali atau setelah reset Redis)
go run cmd/seed/main.go

# buat folder data (wajib ada sebelum pertama kali)
mkdir -p data/stats

# jalankan server
go run main.go

# build lokal
go build -o vidbot-api main.go
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
│     └─ stats.db                 ← stats tracking SQLite
├─ internal/
│  ├─ admin/
│  │  └─ handler.go
│  ├─ auth/
│  │  ├─ handler.go
│  │  └─ service.go
│  ├─ services/
│  │  ├─ content/
│  │  │  ├─ instagram/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  ├─ provider/
│  │  │  │  ├─ downr/
│  │  │  │  │  └─ downr.go
│  │  │  │  ├─ vidown/
│  │  │  │  │  └─ vidown.go
│  │  │  │  └─ provider.go
│  │  │  ├─ spotify/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  ├─ threads/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  ├─ tiktok/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  └─ twitter/
│  │  │     ├─ handler.go
│  │  │     └─ service.go
│  │  ├─ convert/
│  │  │  ├─ audio/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  ├─ document/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  ├─ fonts/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  ├─ image/
│  │  │  │  ├─ handler.go
│  │  │  │  └─ service.go
│  │  │  └─ provider/
│  │  │     ├─ cloudconvert/
│  │  │     │  └─ cloudconvert.go
│  │  │     ├─ convertio/
│  │  │     │  └─ convertio.go
│  │  │     ├─ polling.go
│  │  │     └─ provider.go
│  │  ├─ iptv/
│  │  │  └─ handler.go
│  │  ├─ leakcheck/
│  │  │  └─ handler.go
│  │  └─ vidhub/
│  │     ├─ vidarato/
│  │     │  ├─ handler.go
│  │     │  └─ service.go
│  │     ├─ vidbos/
│  │     │  ├─ handler.go
│  │     │  └─ service.go
│  │     ├─ videb/
│  │     │  ├─ handler.go
│  │     │  └─ service.go
│  │     ├─ vidnest/
│  │     │  ├─ handler.go
│  │     │  └─ service.go
│  │     └─ vidoy/
│  │        ├─ handler.go
│  │        ├─ model.go
│  │        └─ service.go
│  └─ stream/
│     └─ handler.go
├─ middleware/
│  ├─ api_key.go
│  ├─ auth.go
│  ├─ feature.go                  ← feature flag middleware (baru)
│  └─ ratelimit.go
├─ pkg/
│  ├─ apikey/
│  │  └─ types.go
│  ├─ cache/
│  │  ├─ cache.go
│  │  └─ provider_cache.go        ← in-memory provider priority cache (baru)
│  ├─ cloudconvert/
│  │  └─ client.go
│  ├─ convertvalidator/
│  │  └─ validator.go
│  ├─ downloader/
│  │  ├─ cache.go
│  │  ├─ detector.go
│  │  └─ download_url.go
│  ├─ fileutil/
│  │  └─ filename.go              ← sanitize filename unified
│  ├─ httputil/
│  │  └─ json.go                  ← writeJSONUnescaped unified
│  ├─ iptvstore/
│  │  └─ store.go
│  ├─ leakcheck/
│  │  └─ store.go
│  ├─ limiter/
│  │  ├─ global.go
│  │  ├─ limiter.go
│  │  └─ ratelimit.go
│  ├─ mediaresponse/
│  │  ├─ helpers.go
│  │  └─ response.go
│  ├─ proxy/
│  │  ├─ proxy.go
│  │  └─ ua.go
│  ├─ response/
│  │  └─ response.go
│  ├─ stats/
│  │  ├─ db.go                    ← SQLite init + query
│  │  └─ tracker.go               ← async write via buffered channel
│  └─ validator/
│     └─ url.go
├─ router/
│  ├─ router.go                   ← orchestrate only, panggil sub-router
│  ├─ admin.go                    ← route /admin/* (baru)
│  ├─ auth.go                     ← route /auth/*
│  ├─ content.go                  ← route /content/*
│  ├─ convert.go                  ← route /convert/*
│  ├─ iptv.go                     ← route /iptv/*
│  ├─ leakcheck.go                ← route /leakcheck/*
│  └─ vidhub.go                   ← route /vidhub/*
├─ test/
│  ├─ TestNih.jpg
│  └─ TestNih.txt
├─ tools/
│  ├─ Logs/
│  ├─ ffmpeg.exe
│  ├─ ffprobe.exe
│  ├─ N_m3u8DL-RE.exe
│  ├─ shaka-packager.exe
│  └─ yt-dlp.exe
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

### Admin (gunakan Master Key via `X-Master-Key`)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/admin/keys` | Buat API key baru |
| DELETE | `/admin/keys/:key` | Revoke API key |
| GET | `/admin/keys` | List semua API key (`?active=true/false`) |
| POST | `/admin/keys/:key/topup` | Top-up quota |
| GET | `/admin/keys/:key/usage` | Usage detail per API key |
| GET | `/admin/features` | Status semua feature flag |
| GET | `/admin/features/:group/enable` | Nyalakan group |
| GET | `/admin/features/:group/disable` | Matikan group |
| GET | `/admin/features/:group/:platform/enable` | Nyalakan platform |
| GET | `/admin/features/:group/:platform/disable` | Matikan platform |
| GET | `/admin/stats` | Statistik usage seluruh API |

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

| Query Param | Keterangan |
|---|---|
| `key` | API Key (wajib) |
| `country` | Filter negara (opsional) |
| `category` | Filter kategori (opsional) |

Contoh URL langsung di VLC:
```
https://your-domain.com/iptv/playlist?country=ID&key=YOUR_API_KEY
```

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
| POST | `/vidhub/vidarato` | Ekstrak dari Vidarato |
| POST | `/vidhub/vidnest` | Ekstrak dari Vidnest |

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
| GET | `/dl` | Proxy download stream |

---

## Feature Flags

Feature flag memungkinkan enable/disable endpoint tanpa redeploy.
Status disimpan di Redis — efektif langsung tanpa restart server.

### Group Level
```
GET /admin/features/iptv/disable     → matikan semua /iptv/*
GET /admin/features/content/enable   → nyalakan semua /content/*
```

### Platform Level
```
GET /admin/features/content/tiktok/disable   → matikan hanya /content/tiktok
GET /admin/features/vidhub/videb/disable     → matikan hanya /vidhub/videb
GET /admin/features/convert/audio/disable    → matikan hanya /convert/audio
```

### Group yang tersedia
| Group | Platform |
|---|---|
| `content` | `spotify`, `tiktok`, `instagram`, `twitter`, `threads` |
| `vidhub` | `videb`, `vidoy`, `vidbos`, `vidarato`, `vidnest` |
| `convert` | `audio`, `document`, `image`, `fonts` |
| `iptv` | — (group level only) |
| `leakcheck` | — (group level only) |

---

## Stats Tracking

Stats disimpan di SQLite (`data/stats/stats.db`) — tidak di Redis.
Setiap request yang lolos rate limit akan di-track via `stats.Platform()` atau `stats.Group()` di handler.

### Cara Pakai di Handler Baru
```go
import "vidbot-api/pkg/stats"

func (h *Handler) Extract(c *gin.Context) {
    stats.Platform(c, "content", "tiktok") // ← baris pertama
    // ... sisa kode
}

// untuk group tanpa platform (iptv, leakcheck)
stats.Group(c, "iptv")
```

### Response `GET /admin/stats`
```json
{
  "success": true,
  "data": {
    "total_keys": 10,
    "active_keys": 8,
    "total_requests": 1240,
    "today_requests": 45,
    "unique_keys": 6,
    "usage": {
      "content": {
        "platforms": {
          "tiktok": 500,
          "instagram": 300,
          "spotify": 200,
          "twitter": 100,
          "threads": 50
        }
      },
      "iptv": 80,
      "leakcheck": 10
    }
  }
}
```

---

## Arsitektur Provider

### Pattern
Setiap kategori (content, convert, vidhub) menggunakan **provider pattern**:
- Interface `Provider` didefinisikan di folder `provider/`
- Setiap implementasi (downr, cloudconvert, convertio) mengimplementasikan interface
- Service iterate providers dengan fallback otomatis
- Urutan provider diambil dari Redis, bukan hardcode

### Redis Keys — Provider Priority
```
content:provider:spotify    → ["downr"]
content:provider:tiktok     → ["downr", "vidown"]
content:provider:instagram  → ["downr", "vidown"]
content:provider:twitter    → ["downr", "vidown"]
content:provider:threads    → ["downr", "vidown"]
convert:provider:audio      → ["cloudconvert", "convertio"]
convert:provider:document   → ["cloudconvert", "convertio"]
convert:provider:image      → ["cloudconvert", "convertio"]
convert:provider:fonts      → ["cloudconvert", "convertio"]
```

### Ganti provider tanpa redeploy
```bash
# ganti urutan priority convert
DEL convert:provider:audio
RPUSH convert:provider:audio convertio cloudconvert

# ganti untuk content
DEL content:provider:tiktok
RPUSH content:provider:tiktok provider_baru downr
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

---

## Rate Limiting

Rate limit diterapkan per endpoint group via middleware `RateLimit`:

| Group | Limit |
|---|---|
| `/content/*` | 10 req/menit per API key |
| `/convert/*` | 20 req/menit per API key |
| `/vidhub/*` | 30 req/menit per API key |
| `/iptv/*` | 60 req/menit per API key |
| `/leakcheck/*` | 5 req/menit per API key |
| `/iptv/playlist` | 60 req/menit per API key |

Untuk mengubah limit, edit `endpointLimits` di `pkg/limiter/ratelimit.go`.

---

## Cache

Response di-cache di Redis untuk mengurangi hit ke provider eksternal.
`server_1` dan `server_2` **tidak disimpan** di cache — di-generate ulang saat cache hit.

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
| `vidhub:vidarato` | 2 jam |
| `vidhub:vidnest` | 2 jam |

---

## Arsitektur Router

`router/router.go` hanya bertugas sebagai orchestrator — inisialisasi providers dan
memanggil sub-router. Tidak ada route yang didefinisikan langsung di sini.

| File | Tanggung Jawab |
|---|---|
| `router/router.go` | Inisialisasi providers, proxy client, provider cache, panggil sub-router |
| `router/admin.go` | Route `/admin/*` |
| `router/auth.go` | Route `/auth/*` |
| `router/content.go` | Route `/content/*` |
| `router/vidhub.go` | Route `/vidhub/*` |
| `router/convert.go` | Route `/convert/*` |
| `router/iptv.go` | Route `/iptv/*` dan `/iptv/playlist` |
| `router/leakcheck.go` | Route `/leakcheck/*` |

**Aturan:** kalau menambah platform atau provider baru, `router/router.go` dan
`main.go` **tidak perlu disentuh** — cukup file sub-router yang relevan.

---

## Shared Utilities

### `pkg/httputil` — JSON Response
```go
import "vidbot-api/pkg/httputil"

httputil.WriteJSONOK(c, res)              // status 200
httputil.WriteJSON(c, http.StatusOK, res) // status custom
```
Mencegah `\u0026` pada URL di dalam response JSON.

### `pkg/fileutil` — Sanitize Filename
```go
import "vidbot-api/pkg/fileutil"

filename := fileutil.Sanitize(title) + ".mp4"
filename := fileutil.SanitizeWithExt(rawName, ext)
```

### `pkg/cache/provider_cache` — Provider Priority Cache
```go
// Inisialisasi di router/router.go — otomatis sync dari Redis setiap 5 menit
cache.InitProviderCache([]string{
    "content:provider:tiktok",
    // ...
})

// Tidak perlu dipanggil manual di tempat lain
// ResolveProviderForCategory sudah pakai GetProviderOrder() secara otomatis
```
Provider priority dibaca dari memory, bukan Redis, sehingga tidak ada round-trip Redis per request. Kalau urutan provider diubah di Redis (via `DEL` + `RPUSH`), perubahan akan aktif dalam maksimal 5 menit tanpa restart server.

### `pkg/stats` — Stats Tracking
```go
import "vidbot-api/pkg/stats"

stats.Platform(c, "content", "tiktok") // untuk endpoint dengan platform
stats.Group(c, "iptv")                 // untuk endpoint tanpa platform
```

---

## Konvensi Kode

### Menambah platform content baru (misal: YouTube)

```
1. Buat folder baru:
   internal/services/content/youtube/
   ├─ handler.go   ← ikuti pola spotify/handler.go
   └─ service.go

2. pkg/downloader/cache.go
   → tambah entry TTL di cacheTTL map

3. router/content.go
   → tambah provider slice, handler, route, dan FeatureFlagPlatform

4. cmd/seed/main.go
   → tambah allowed_domains + content:provider key

5. config/allowed_domains.json
   → tambah domain list

6. internal/admin/handler.go
   → tambah "youtube" ke validPlatforms["content"]

7. Di handler baru, tambah di baris pertama Extract():
   stats.Platform(c, "content", "youtube")
```

### Menambah provider content baru (misal: Cobalt)

```
1. Buat folder baru:
   internal/services/content/provider/cobalt/
   └─ cobalt.go   ← implementasi interface Name() + Extract()

2. router/content.go
   → tambah cobalt ke slice provider yang relevan

3. cmd/seed/main.go
   → tambah "cobalt" ke content:provider:* key yang relevan

4. router/router.go
   → tambah key "content:provider:cobalt" di slice InitProviderCache
```

### Menambah platform vidhub baru (misal: Vidplay)

```
1. Buat folder baru:
   internal/services/vidhub/vidplay/
   ├─ handler.go   ← ikuti pola vidbos/handler.go
   └─ service.go

2. pkg/downloader/cache.go
   → tambah entry TTL di cacheTTL map

3. router/vidhub.go
   → tambah handler, route, dan FeatureFlagPlatform

4. cmd/seed/main.go
   → tambah allowed_domains key

5. config/allowed_domains.json
   → tambah domain list

6. internal/admin/handler.go
   → tambah "vidplay" ke validPlatforms["vidhub"]

7. Di handler baru, tambah di baris pertama Extract():
   stats.Platform(c, "vidhub", "vidplay")
```

### Menambah format convert baru

```
1. Tambah di allowedFormats di service.go kategori yang relevan
2. Tambah di formatCompatibility map
3. Tambah content type di pkg/convertvalidator/validator.go
4. Tambah di supportedFormats di cloudconvert.go dan convertio.go
```

### Menambah provider convert baru

```
1. Buat folder baru:
   internal/services/convert/provider/{nama}/

2. Implementasikan interface Provider:
   Name(), SupportedFormats(), Submit(), SubmitUpload(), Status()

3. router/convert.go
   → tambah ke slice convertProviders
```

### Cheatsheet ringkas

| Skenario | File yang disentuh |
|---|---|
| Platform content baru | `content/{nama}/` + `cache.go` + `router/content.go` + `seed` + `allowed_domains.json` + `admin/handler.go` |
| Provider content baru | `content/provider/{nama}/` + `router/content.go` + `seed` + `router/router.go` (InitProviderCache) |
| Platform vidhub baru | `vidhub/{nama}/` + `cache.go` + `router/vidhub.go` + `seed` + `allowed_domains.json` + `admin/handler.go` |
| Format convert baru | `service.go` + `validator.go` + `cloudconvert.go` + `convertio.go` |
| Provider convert baru | `convert/provider/{nama}/` + `router/convert.go` |

### Response
- Selalu gunakan `httputil.WriteJSONOK` — jangan `c.JSON()` untuk response yang mengandung URL
- Error response selalu via `response.ErrorWithCode(c, status, "CODE", "message")`
- Cache selalu disimpan **tanpa** `server_1` dan `server_2`
- Tambah `stats.Platform()` atau `stats.Group()` di baris pertama setiap handler baru

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

## Known Bugs & Status

| # | Bug | File | Status |
|---|---|---|---|
| 1 | `/convert/image/upload` validasi pakai `Audio` bukan `Image` | `internal/services/convert/image/handler.go` | ✅ Fixed |
| 2 | `content:threads` tidak ada di `cacheTTL`, fallback ke 15 menit | `pkg/downloader/cache.go` | ✅ Fixed |
| 3 | `iptvstore.startRefresh()` tidak dipanggil di `Init()` | `pkg/iptvstore/store.go` | ✅ Fixed |
| 4 | Goroutine secondary di content service tidak ada context cancellation | `internal/services/content/*/service.go` | 🟡 Low priority |

---

## Pending / Backlog

- [ ] Health check endpoint (`GET /health`) ✅ Selesai
- [ ] Structured logging ke file
- [ ] Tier sistem (free, pro, enterprise) untuk rate limit + quota berbeda
- [ ] Fix ID dan Duration kosong di response TikTok
- [ ] CF Worker: tambah Referer header untuk Convertio URLs (server_1 masih 403)
- [ ] Cache hasil convert untuk hemat credits CloudConvert/Convertio
- [ ] Dokumentasi API publik (Postman collection atau README terpisah)
- [ ] Provider priority pindah ke memory (kurangi hit Redis) ✅ Selesai
- [ ] URL versioning `/v1/`
- [ ] Konsolidasi `writeJSONUnescaped` ke `pkg/httputil` ✅ Selesai
- [ ] Konsolidasi `sanitizeFilename` ke `pkg/fileutil` ✅ Selesai
- [ ] Pecah `router/router.go` ke sub-router per grup ✅ Selesai
- [ ] Graceful shutdown ✅ Selesai
- [ ] Feature flag per group dan platform ✅ Selesai
- [ ] Stats tracking per platform via SQLite ✅ Selesai
- [ ] IPTV playlist endpoint (`GET /iptv/playlist`) ✅ Selesai
- [ ] IPTV stream format detection (`format` field) ✅ Selesai
- [ ] gin.ReleaseMode di production ✅ Selesai
- [ ] Async stats write via buffered channel ✅ Selesai
- [ ] Convertvalidator timeout turun ke 3 detik
- [ ] Cleanup stats scheduler (hapus data > 90 hari)
- [ ] Structured logging via slog

---

## Keputusan Teknis

| Keputusan | Alasan |
|---|---|
| Provider pattern dengan Redis priority | Ganti provider tanpa redeploy |
| `from` wajib di convert | Cegah hit provider untuk kombinasi format yang tidak support, hemat credits |
| Cache tanpa server_1/server_2 | URL download berisi HMAC yang time-based, tidak bisa disimpan permanen |
| Hex encoding untuk download URL | Karakter aman, tidak ada padding atau karakter spesial |
| `httputil.WriteJSONOK` | Mencegah `\u0026` di URL dalam response JSON, satu implementasi untuk semua handler |
| `fileutil.Sanitize` | Satu implementasi sanitize filename, mencegah perbedaan behavior antar service |
| Sub-router per grup | `router/router.go` tidak perlu disentuh saat tambah platform baru |
| Rate limit per group via Redis | Bisa diubah tanpa redeploy, state tersimpan across instance |
| Stats tracking via SQLite | Mengurangi hit Redis, data permanen tidak hilang saat Redis di-flush |
| Feature flag via Redis | Real-time toggle tanpa restart server |
| SMIL URL tidak di-resolve ke chunklist | Chunklist berisi token dinamis yang expire — VLC fetch sendiri dengan header yang benar |
| Graceful shutdown 30 detik | HLS download bisa makan waktu lama, tidak boleh terputus paksa |
| IPTV playlist auth via `?key=` | Player seperti VLC tidak bisa kirim custom header |
| `gin.ReleaseMode` di production | Matikan debug logging, kurangi I/O overhead di traffic tinggi |
| Async stats write via buffered channel | Hilangkan SQLite write latency dari request path, stats boleh tidak 100% akurat |
| Provider priority di memory (`pkg/cache/provider_cache.go`) | Hilangkan Redis round-trip per request, sync dari Redis setiap 5 menit |