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
| Cache / State | Redis (Upstash) |
| Stats Tracking | SQLite (lokal, `data/stats/stats.db`) |
| App Store | SQLite per platform (lokal, `data/app/{platform}.db`) |
| Proxy Layer | Cloudflare Workers (3 worker berbeda) |
| Auth | Time-based HMAC token + API Key |
| File Conversion | CloudConvert, Convertio |
| Media Extraction | Downr + Vidown (via CF Worker) |
| HLS Processing | ffmpeg (pipe stdin→stdout, tidak ada tmp file) |
| HLS Fallback | yt-dlp (fallback kalau direct HLS gagal) |

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
│  ├─ health/
│  │  └─ handler.go               ← health check semua dependencies
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
│  │  ├─ app/
│  │  │  ├─ handler.go        ← SearchAndroid, SearchWindows, Download
│  │  │  └─ admin_handler.go  ← AdminAdd, AdminBulkAdd, AdminList, AdminDelete, AdminDeleteVersion
│  │  └─ vidhub/
│  │     ├─ kingbokeptv/           ← HLS site, pakai GenerateServer*HLSURL
│  │     │  ├─ handler.go
│  │     │  └─ service.go
│  │     ├─ vidarato/              ← HLS site (master playlist 2 level), pakai GenerateServer*HLSURL
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
│     └─ handler.go               ← HLS progressive download + direct stream
├─ middleware/
│  ├─ api_key.go
│  ├─ auth.go
│  ├─ feature.go                  ← feature flag middleware
│  └─ ratelimit.go
├─ pkg/
│  ├─ apikey/
│  │  └─ types.go
│  ├─ cache/
│  │  ├─ cache.go
│  │  └─ provider_cache.go        ← in-memory provider priority cache
│  ├─ cloudconvert/
│  │  └─ client.go
│  ├─ convertvalidator/
│  │  └─ validator.go
│  ├─ downloader/
│  │  ├─ cache.go
│  │  ├─ detector.go
│  │  └─ download_url.go          ← GenerateServer*URL + GenerateServer*HLSURL + ExtractCDNOrigin
│  ├─ fileutil/
│  │  └─ filename.go              ← sanitize filename unified
│  ├─ httputil/
│  │  └─ json.go                  ← writeJSONUnescaped unified
│  ├─ iptvstore/
│  │  └─ store.go
│  ├─ leakcheck/
│  │  └─ store.go
│  ├─ appstore/
│  │  ├─ db.go               ← SQLite per platform, FTS5, write+read DB terpisah
│  │  └─ shortlink.go        ← mask/resolve raw URL via Redis (prefix app:sl:)
│  ├─ limiter/
│  │  ├─ global.go                ← HLSDownload(3), DirectStream(10), cdnMaxPerHost(1)
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
│  ├─ shortlink/
│  │  └─ shortlink.go             ← server_2 URL shortener via Redis
│  ├─ stats/
│  │  ├─ db.go                    ← SQLite init + query
│  │  └─ tracker.go               ← async write via buffered channel
│  └─ validator/
│     └─ url.go
├─ router/
│  ├─ router.go                   ← orchestrate only, panggil sub-router
│  ├─ admin.go                    ← route /admin/*
│  ├─ auth.go                     ← route /auth/*
│  ├─ content.go                  ← route /content/*
│  ├─ convert.go                  ← route /convert/*
│  ├─ health.go                   ← route /health
│  ├─ iptv.go                     ← route /iptv/*
│  ├─ leakcheck.go                ← route /leakcheck/*
│  ├─ app.go                      ← route /app/* dan /admin/app/*
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
| POST | `/app/android` | Cari APK Android (keyword via `apk`) |
| POST | `/app/windows` | Cari software Windows (keyword via `app`) |
| GET | `/app/dl?k={key}` | Redirect ke raw URL via shortlink — **tidak butuh auth** |

#### Request body
```json
// android
{"apk": "classical music"}

// windows
{"app": "Internet Download Manager"}
```
Keyword wajib diisi, minimal 3 karakter. Menggunakan FTS5 dengan fallback LIKE.

### Admin App (gunakan Master Key via `X-Master-Key`)
| Method | Path | Keterangan |
|---|---|---|
| GET | `/admin/app/:platform/list` | List semua app (`?q=keyword` opsional) |
| POST | `/admin/app/:platform/add` | Tambah satu entry |
| POST | `/admin/app/:platform/bulk` | Tambah banyak entry (max 200) |
| DELETE | `/admin/app/:platform/:slug` | Hapus app beserta semua versinya |
| DELETE | `/admin/app/:platform/version/:id` | Hapus satu versi download |

`:platform` yang valid: `android`, `windows`

### App (butuh API Key + Access Token)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/app/android` | Cari app Android (keyword via `apk`) |
| POST | `/app/windows` | Cari app Windows (keyword via `app`) |
| GET | `/app/dl?k={key}` | Redirect ke raw URL via shortlink — tidak butuh auth |

### Admin App (gunakan Master Key via `X-Master-Key`)
| Method | Path | Keterangan |
|---|---|---|
| GET | `/admin/app/{platform}/list` | List semua app platform tertentu (`?q=keyword`) |
| POST | `/admin/app/{platform}/add` | Tambah satu entry |
| POST | `/admin/app/{platform}/bulk` | Tambah banyak entry (maks 200) |
| DELETE | `/admin/app/{platform}/{slug}` | Hapus app beserta semua versinya |
| DELETE | `/admin/app/{platform}/version/{id}` | Hapus satu versi download |

Platform yang tersedia: `android`, `windows`

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
| `vidhub` | `videb`, `vidoy`, `vidbos`, `vidarato`, `vidnest`, `kingbokeptv` |
| `convert` | `audio`, `document`, `image`, `fonts` |
| `app` | `android`, `windows` |
| `iptv` | — (group level only) |
| `leakcheck` | — (group level only) |
| `app` | `android`, `windows` |

---

## Stats Tracking

Stats disimpan di SQLite (`data/stats/stats.db`) — tidak di Redis.
Write dilakukan async via buffered channel (kapasitas 2000) — tidak memblokir request path.
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
Setiap kategori (content, convert) menggunakan **provider pattern**:
- Interface `Provider` didefinisikan di folder `provider/`
- Setiap implementasi (downr, cloudconvert, convertio) mengimplementasikan interface
- Service iterate providers dengan fallback otomatis
- Urutan provider diambil dari memory cache (`pkg/cache/provider_cache.go`), sync dari Redis setiap 5 menit

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

Perubahan aktif dalam maksimal 5 menit (interval sync provider cache).

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
| `/app/*` | 30 req/menit per API key |

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
| `vidhub:vidarato` | 3 menit (token URL expire & IP-bound) |
| `vidhub:vidnest` | 2 jam |
| `vidhub:kingbokeptv` | 6 jam |

**Catatan vidarato:** TTL sengaja 3 menit karena `streaming_url` mengandung token yang terikat ke IP client dan expire time. Cache hampir selalu miss — pertimbangkan untuk skip cache di handler vidarato sepenuhnya di masa depan.

---

## Arsitektur HLS Progressive Download

Site yang mengembalikan m3u8 (kingbokeptv, vidarato) diproses via `internal/stream/handler.go` menggunakan arsitektur progressive download.

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
    ↓
Client baca chunks progressive (streaming ke browser/IDM)
```

### Concurrency Limits
| Limiter | Nilai | Keterangan |
|---|---|---|
| `HLSDownload` | 3 | Max concurrent HLS session baru |
| `DirectStream` | 10 | Max concurrent direct MP4 relay |
| `cdnMaxPerHost` | 1 | Max concurrent download ke CDN host yang sama |

### Session Lifecycle
- Session di-share antar user yang request URL sama (tidak spawn 2 ffmpeg)
- Limiter `HLSDownload` di-release setelah session done (bukan setelah handler return)
- Chunks di-free dari memory segera setelah semua reader selesai + download done
- Grace period 2 menit untuk reader reconnect sebelum session di-cancel
- Session TTL 10 menit — dihapus dari map setelah expired
- **Tidak ada tmp file di disk** — semua data mengalir lewat pipe ke memory

### Master Playlist Detection
`fetchPlaylist` otomatis handle dua struktur berbeda:
- **1 level** (kingbokeptv): `playlist.m3u8` → langsung berisi segment `.ts`
- **2 level** (vidarato): `master.m3u8` → berisi `index_608x1080.m3u8` → berisi segment `.ts`

### CDNOrigin
`CDNOrigin` (scheme + host CDN) di-extract dari m3u8 URL di service, di-embed ke dalam payload terenkripsi server_2, dan dipakai di stream handler untuk set `Origin` dan `Referer` header yang benar saat download segments. Tidak muncul di response JSON.

Untuk site baru yang punya CDN berbeda, tidak perlu update mapping static — CDNOrigin otomatis di-derive dari URL m3u8 yang di-scrape.

### Fungsi Generate URL untuk HLS

Site yang output-nya m3u8 **wajib** pakai fungsi HLS khusus (bukan fungsi regular):

```go
// ✅ untuk site HLS (kingbokeptv, vidarato, site baru yang m3u8)
res.Download.Server1 = downloader.GenerateServer1HLSURL(..., result.CDNOrigin)
res.Download.Server2 = downloader.GenerateServer2HLSURL(..., result.CDNOrigin)

// ✅ untuk site non-HLS (videb, vidoy, vidbos, vidnest, semua content)
res.Download.Server1 = downloader.GenerateServer1URL(...)
res.Download.Server2 = downloader.GenerateServer2URL(...)
```

Perbedaannya: fungsi HLS membawa `CDNOrigin` di dalam payload terenkripsi untuk dipakai stream handler. Fungsi regular tidak.

---

## Arsitektur Router

`router/router.go` hanya bertugas sebagai orchestrator — inisialisasi providers dan
memanggil sub-router. Tidak ada route yang didefinisikan langsung di sini.

| File | Tanggung Jawab |
|---|---|
| `router/router.go` | Inisialisasi providers, proxy client, provider cache, shortlink wiring, panggil sub-router |
| `router/admin.go` | Route `/admin/*` |
| `router/auth.go` | Route `/auth/*` |
| `router/health.go` | Route `/health` |
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

**Penting:** Selalu pakai `httputil.WriteJSONOK` atau `httputil.WriteJSON` untuk response yang mengandung URL. Jangan pakai `c.JSON()` atau `response.WriteJSON()` — keduanya melakukan HTML escaping yang merusak URL.

### `pkg/fileutil` — Sanitize Filename
```go
import "vidbot-api/pkg/fileutil"

filename := fileutil.Sanitize(title) + ".mp4"
filename := fileutil.SanitizeWithExt(rawName, ext)
```

### `pkg/downloader` — URL Generation & Cache
```go
import "vidbot-api/pkg/downloader"

// untuk site non-HLS
downloader.GenerateServer1URL(workerURL, secret, xorKey, dlURL, title, filename, filecode, ext, service)
downloader.GenerateServer2URL(appURL, streamSecret, cacheKey, dlURL, title, filename, filecode, ext, service)

// untuk site HLS (output m3u8)
downloader.GenerateServer1HLSURL(..., cdnOrigin)
downloader.GenerateServer2HLSURL(..., cdnOrigin)

// extract CDN origin dari m3u8 URL (dipakai di service.go dan cache hit handler)
cdnOrigin := downloader.ExtractCDNOrigin(m3u8URL)

// cache
downloader.CacheGet[T](service, site, rawURL)
downloader.CacheSet(service, site, rawURL, &data)
downloader.CacheKey(service, site, rawURL)
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

### Menambah platform vidhub baru — Non-HLS (misal: Vidplay)

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

8. Gunakan GenerateServer1URL / GenerateServer2URL (bukan versi HLS)
```

### Menambah platform vidhub baru — HLS/M3U8 (misal: SiteStreamBaru)

```
1. Buat folder baru:
   internal/services/vidhub/sitestreambar/
   ├─ handler.go   ← ikuti pola kingbokeptv/handler.go
   └─ service.go

2. Di service.go:
   → Extract m3u8 URL dari halaman
   → Tambah CDNOrigin di struct Result:
      CDNOrigin string
   → Isi di return:
      CDNOrigin: extractCDNOrigin(m3u8URL)
   → Tambah fungsi helper:
      func extractCDNOrigin(m3u8URL string) string {
          parsed, err := url.Parse(m3u8URL)
          if err != nil || parsed.Host == "" { return "" }
          return parsed.Scheme + "://" + parsed.Host
      }

3. Di handler.go:
   → Pakai GenerateServer1HLSURL / GenerateServer2HLSURL (bukan versi regular)
   → Saat cache hit: cdnOrigin := downloader.ExtractCDNOrigin(cached.Download.Original)
   → Saat build response: JANGAN masukkan CDNOrigin ke VidhubData (tidak perlu, tidak boleh muncul di response)

4. pkg/downloader/cache.go
   → tambah entry TTL di cacheTTL map

5. router/vidhub.go
   → tambah handler, route, dan FeatureFlagPlatform

6. cmd/seed/main.go + config/allowed_domains.json
   → tambah allowed_domains

7. internal/admin/handler.go
   → tambah ke validPlatforms["vidhub"]

8. Di handler baru, tambah di baris pertama Extract():
   stats.Platform(c, "vidhub", "sitestreambar")
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

### Menambah platform app baru (misal: macOS)

```
1. Tambah di validPlatforms di pkg/appstore/db.go:
   "macos": true,

2. pkg/appstore/db.go — Init() otomatis buat macos.db saat server start

3. internal/services/app/handler.go
   → tambah SearchMacOS() ikuti pola SearchAndroid/SearchWindows

4. router/app.go
   → tambah route POST /app/macos + FeatureFlagPlatform("app", "macos")

5. internal/admin/handler.go
   → tambah "macos" ke validPlatforms["app"]

6. pkg/limiter/ratelimit.go — tidak perlu diubah, sudah pakai group "app"
```

### Cheatsheet ringkas (update)

| Skenario | File yang disentuh |
|---|---|
| Platform content baru | `content/{nama}/` + `cache.go` + `router/content.go` + `seed` + `allowed_domains.json` + `admin/handler.go` |
| Provider content baru | `content/provider/{nama}/` + `router/content.go` + `seed` + `router/router.go` (InitProviderCache) |
| Platform vidhub baru (non-HLS) | `vidhub/{nama}/` + `cache.go` + `router/vidhub.go` + `seed` + `allowed_domains.json` + `admin/handler.go` |
| Platform vidhub baru (HLS) | sama seperti non-HLS + pakai `GenerateServer*HLSURL` + `extractCDNOrigin` di service |
| Format convert baru | `service.go` + `validator.go` + `cloudconvert.go` + `convertio.go` |
| Provider convert baru | `convert/provider/{nama}/` + `router/convert.go` |
| Platform app baru (misal macOS) | `validPlatforms` di `db.go` + handler baru + `router/app.go` + `admin/handler.go` |

### Response
- Selalu gunakan `httputil.WriteJSONOK` atau `httputil.WriteJSON` — **jangan** `c.JSON()` atau `response.WriteJSON()` untuk response yang mengandung URL
- Error response selalu via `response.ErrorWithCode(c, status, "CODE", "message")`
- Cache selalu disimpan **tanpa** `server_1` dan `server_2`
- Tambah `stats.Platform()` atau `stats.Group()` di baris pertama setiap handler baru
- Site HLS wajib pakai `GenerateServer*HLSURL`, site non-HLS pakai `GenerateServer*URL`

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
Module untuk menyimpan dan mencari APK/software modded.
Data disimpan di SQLite terpisah per platform (`data/app/{platform}.db`).
Raw URL download di-mask via Redis shortlink (TTL 72 jam, auto-refresh saat di-search).
Pencarian menggunakan FTS5 dengan fallback LIKE.

### Struktur Data
```
data/
└─ app/
   ├─ android.db    ← apps + app_downloads + apps_fts (FTS5)
   └─ windows.db    ← sama, terpisah per platform
```

### File yang Terlibat
```
pkg/appstore/
├─ db.go         ← SQLite store: Init, Search, SearchAll, Upsert, Delete, DeleteVersion
└─ shortlink.go  ← MaskURL / ResolveURL via Redis (prefix app:sl:)

internal/services/app/
├─ handler.go        ← SearchAndroid, SearchWindows, Download
└─ admin_handler.go  ← AdminAdd, AdminBulkAdd, AdminList, AdminDelete, AdminDeleteVersion

router/app.go        ← sub-router /app/* dan /admin/app/*
```

### Konvensi
- Keyword wajib diisi minimal 3 karakter di public endpoint — tolak kalau kosong
- Admin list (`/admin/app/{platform}/list`) boleh tanpa keyword — pakai `SearchAll`
- Platform diambil dari URL param `/:platform` — bukan dari body
- `normPlatform` harus return error 400 kalau platform tidak valid, jangan silent fallback ke android
- FTS5 dengan `LIMIT 50` — jangan query tanpa limit
- N+1 query `getDownloads` — perlu diganti JOIN atau `IN (...)` saat data sudah besar

### Known Issues & Backlog Appstore
| # | Issue | Priority |
|---|---|---|
| A1 | `SetMaxOpenConns(1)` untuk semua operasi — pisahkan read/write pool | 🔴 High |
| A2 | N+1 query `getDownloads` per app — ganti ke single JOIN query | 🔴 High |
| A3 | `hashKey` di shortlink.go pakai XOR fold — collision-prone, ganti ke sha256 | 🔴 High |
| A4 | `normPlatform` silent fallback ke android — harus return 400 kalau invalid | 🟡 Medium |
| A5 | Shortlink TTL 72 jam terlalu pendek untuk URL statis APK/EXE | 🟡 Medium |
| A6 | `SearchAll` tanpa pagination — OOM risk saat data ribuan | 🟡 Medium |
| A7 | `migrateDB` multiple statement dalam satu Exec — pisah per statement | 🟡 Medium |
| A8 | Tidak ada `rows.Err()` check setelah loop di semua fungsi query | 🟡 Medium |
| A9 | FTS query tanpa LIMIT — tambah `LIMIT 50` | 🟡 Medium |
| A10 | Tidak ada WAL checkpoint berkala — WAL file bisa tumbuh tak terbatas | 🟡 Medium |
| A11 | Tidak ada FTS AFTER UPDATE trigger — kalau ada edit nama, index stale | 🟡 Medium |
| A12 | `toSlug` tidak handle collision — dua nama bisa generate slug sama | 🟡 Medium |
| A13 | `validPlatforms` hardcoded — tambah platform baru butuh rebuild | 🟢 Low |
| A14 | Tidak ada `stats.Platform` di admin add/bulk | 🟢 Low |
| A15 | Tidak ada mekanisme bulk import massal (>200) dengan batch FTS rebuild | 🟢 Low |

### Menambah Platform App Baru (misal: macOS)
```
1. pkg/appstore/db.go
   → tambah "macos": true di validPlatforms

2. router/app.go
   → tambah route POST /app/macos + handler SearchMacOS

3. internal/services/app/handler.go
   → tambah func SearchMacOS

4. internal/admin/handler.go
   → tambah "macos" ke validPlatforms["app"]

5. pkg/limiter/ratelimit.go — tidak perlu, sudah pakai group "app"
```

---

| # | Bug | File | Status |
|---|---|---|---|
| 1 | `/convert/image/upload` validasi pakai `Audio` bukan `Image` | `internal/services/convert/image/handler.go` | ✅ Fixed |
| 2 | `content:threads` tidak ada di `cacheTTL`, fallback ke 15 menit | `pkg/downloader/cache.go` | ✅ Fixed |
| 3 | `iptvstore.startRefresh()` tidak dipanggil di `Init()` | `pkg/iptvstore/store.go` | ✅ Fixed |
| 4 | `deriveOrigin` dead code tidak dipakai | `internal/stream/handler.go` | ✅ Fixed (dihapus) |
| 5 | Limiter `HLSDownload` di-release terlalu cepat (sebelum session done) | `internal/stream/handler.go` | ✅ Fixed |
| 6 | Log ffmpeg/yt-dlp terlalu verbose di production | `internal/stream/handler.go` | ✅ Fixed (filter error only) |
| 7 | Master playlist vidarato tidak di-resolve ke sub-playlist | `internal/stream/handler.go` | ✅ Fixed (`isMasterPlaylist` + `fetchM3U8Body`) |
| 8 | HLS session chunks tidak di-free setelah selesai (memory leak) | `internal/stream/handler.go` | ✅ Fixed (`freeChunks()`) |
| 9 | Beberapa vidhub handler pakai `response.WriteJSON` bukan `httputil.WriteJSONOK` | `vidbos`, `videb`, `vidoy`, `vidarato` handler | 🟡 Perlu fix — bisa menyebabkan URL corrupt |
| 10 | Goroutine secondary di content service tidak ada context cancellation | `internal/services/content/*/service.go` | 🟡 Low priority |
| 11 | `vidnest/service.go` masih ada timing log verbose | `internal/services/vidhub/vidnest/service.go` | 🟡 Perlu dihapus |
| 12 | `appstore` — `SetMaxOpenConns(1)` untuk read — antri kalau banyak concurrent search | `pkg/appstore/db.go` | 🔴 Perlu pisah writeDB + readDB |
| 13 | `appstore` — tidak ada `PRAGMA foreign_keys=ON` → CASCADE delete tidak jalan | `pkg/appstore/db.go` | 🔴 Perlu fix |
| 14 | `appstore` — `Upsert` tidak dalam transaksi → bisa partial insert (app tanpa download) | `pkg/appstore/db.go` | 🔴 Perlu fix |
| 15 | `appstore` — `toSlug` tidak handle collision → UNIQUE constraint error kalau slug sama | `pkg/appstore/db.go` | 🟡 Perlu fallback append angka |
| 16 | `appstore` — tidak ada trigger `apps_au` (after update) → FTS index stale kalau data diedit | `pkg/appstore/db.go` | 🟡 Perlu ditambah |
| 17 | `appstore` — `migrateDB` semua DDL dalam satu `Exec` → bisa silent fail di modernc | `pkg/appstore/db.go` | 🟡 Pisah per statement |
| 18 | `appstore` — shortlink `hashKey` pakai XOR fold → collision rate tinggi untuk ribuan URL | `pkg/appstore/shortlink.go` | 🟡 Ganti sha256 |
| 19 | `appstore` — shortlink TTL 72 jam terlalu pendek untuk URL statis | `pkg/appstore/shortlink.go` | 🟡 Naikkan ke 30 hari |
| 20 | `appstore` — `SearchAll` tanpa LIMIT → bisa block koneksi untuk ribuan data | `pkg/appstore/db.go` | 🟡 Tambah pagination |
| 21 | `appstore` — route `DELETE /:platform/:slug` vs `/:platform/version/:id` rawan konflik di gin | `router/app.go` | 🟡 Pisah prefix |
| 22 | `appstore` — `normPlatform` fallback diam-diam ke android kalau platform tidak valid | `internal/services/app/admin_handler.go` | 🟡 Return 400 error |
| 23 | `appstore` — tidak ada `mmap_size` PRAGMA → read performance tidak optimal | `pkg/appstore/db.go` | 🟡 Tambah 256MB |

---

## Pending / Backlog

- [ ] Fix `response.WriteJSON` → `httputil.WriteJSONOK` di vidbos, videb, vidoy, vidarato handler
- [ ] Hapus timing log verbose di `vidnest/service.go`
- [ ] [appstore] Pisah writeDB + readDB di `pkg/appstore/db.go` (sama seperti leakcheck)
- [ ] [appstore] Tambah `PRAGMA foreign_keys=ON` agar CASCADE delete jalan
- [ ] [appstore] Bungkus `Upsert` dalam transaksi (insert app + version atomik)
- [ ] [appstore] Fix `toSlug` collision — fallback append angka kalau slug sudah ada
- [ ] [appstore] Tambah trigger `apps_au` (after update) untuk FTS consistency
- [ ] [appstore] Pisah DDL per `Exec` di `migrateDB`
- [ ] [appstore] Ganti `hashKey` XOR fold → sha256 di `shortlink.go`
- [ ] [appstore] Naikkan shortlink TTL dari 72 jam ke 30 hari
- [ ] [appstore] Tambah pagination di `SearchAll` untuk admin list
- [ ] [appstore] Fix route konflik `/:platform/:slug` vs `/:platform/version/:id`
- [ ] [appstore] `normPlatform` return 400 kalau platform tidak valid, bukan silent fallback
- [ ] [appstore] Tambah `PRAGMA mmap_size=268435456` untuk read performance
- [ ] [appstore] Tambah `LIMIT` di FTS query (default 20) + info pagination di response
- [ ] Structured logging ke file
- [ ] Tier sistem (free, pro, enterprise) untuk rate limit + quota berbeda
- [ ] Fix ID dan Duration kosong di response TikTok
- [ ] CF Worker: tambah Referer header untuk Convertio URLs (server_1 masih 403)
- [ ] Cache hasil convert untuk hemat credits CloudConvert/Convertio
- [ ] Dokumentasi API publik (Postman collection atau README terpisah)
- [ ] URL versioning `/v1/`
- [ ] Convertvalidator timeout turun ke 3 detik
- [ ] Cleanup stats scheduler (hapus data > 90 hari)
- [ ] Structured logging via slog
- [ ] Skip cache sepenuhnya untuk vidarato (token IP-bound, 3 menit cache hampir selalu miss)
- [ ] Health check endpoint (`GET /health`) ✅ Selesai
- [ ] Provider priority pindah ke memory (kurangi hit Redis) ✅ Selesai
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
- [ ] kingbokeptv vidhub endpoint ✅ Selesai
- [ ] HLS progressive download dengan ffmpeg pipe (no tmp file) ✅ Selesai
- [ ] Master playlist auto-resolve (support vidarato 2-level HLS) ✅ Selesai
- [ ] CDNOrigin di payload HLS (tidak muncul di response) ✅ Selesai
- [ ] HLS session memory leak fix (freeChunks setelah done) ✅ Selesai
- [ ] HLS limiter leak fix (release setelah session done bukan handler return) ✅ Selesai
- [ ] Natural delay antar segment HLS (anti throttle 300-800ms) ✅ Selesai
- [ ] CDN concurrent limiter per host (max 1) ✅ Selesai

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
| HLS pipe via ffmpeg stdin/stdout | Tidak ada tmp file di disk, stream langsung ke memory chunks |
| Session sharing untuk HLS | URL yang sama tidak spawn 2 proses ffmpeg, semua client share chunks yang sama |
| CDNOrigin di payload HLS terenkripsi | Tidak perlu mapping static di code, otomatis dari URL yang di-scrape, tidak expose di response |
| `GenerateServer*HLSURL` terpisah dari `GenerateServer*URL` | Site non-HLS tidak terpengaruh saat menambah field CDNOrigin, zero breaking change |
| `freeChunks()` setelah semua reader done | Bebaskan memory segera, tidak tunggu sessionTTL 10 menit |
| `cdnMaxPerHost = 1` | Satu concurrent download per CDN host, lebih aman dari throttle |
| Natural delay 300-800ms antar segment | Simulasi pola browser streaming, kurangi deteksi bot oleh Cloudflare |
| Vidarato cache TTL 3 menit | Token URL expire dan IP-bound, cache lebih lama tidak berguna |
| DB SQLite terpisah per platform (android.db, windows.db) | Isolasi data, query tidak perlu filter platform, mudah backup per platform |
| FTS5 untuk search app dengan fallback LIKE | Performa jauh lebih baik untuk ribuan data, fallback jaga kompatibilitas DB lama |
| Shortlink app pakai prefix `app:sl:` terpisah dari `sl:` | Hindari collision dengan shortlink video, TTL dan payload berbeda |
| Keyword wajib minimal 3 karakter di `/app/*` | Cegah full-scan query yang membebani DB untuk ribuan data |
| `SearchAll` tanpa keyword hanya untuk admin | Public endpoint wajib pakai keyword — admin boleh list semua untuk maintenance |
| `DATA_DIR` menggantikan `LEAKCHECK_DIR` di config | Universal untuk semua module DB (leakcheck, stats, appstore), tidak perlu env var baru per module |