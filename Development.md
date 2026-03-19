# vidbot-api — Development Reference

> Dokumen ini adalah single source of truth untuk pengembangan vidbot-api.
> Update dokumen ini setiap kali ada perubahan arsitektur, endpoint baru, atau keputusan teknis.

---

## Stack

| Komponen | Teknologi |
|---|---|
| Language | Go (gin-gonic) |
| Cache / State | Redis (Upstash) |
| Proxy Layer | Cloudflare Workers |
| Auth | Time-based HMAC token + API Key |
| File Conversion | CloudConvert, Convertio |
| Media Extraction | Downr (via CF Worker) |

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

# jalankan server
go run main.go

# build
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
│  └─ ratelimit.go
├─ pkg/
│  ├─ apikey/
│  │  └─ types.go
│  ├─ cache/
│  │  └─ cache.go
│  ├─ cloudconvert/
│  │  └─ client.go
│  ├─ convertvalidator/
│  │  └─ validator.go
│  ├─ downloader/
│  │  ├─ cache.go
│  │  ├─ detector.go
│  │  └─ download_url.go
│  ├─ fileutil/
│  │  └─ filename.go          ← sanitize filename unified (baru)
│  ├─ httputil/
│  │  └─ json.go              ← writeJSONUnescaped unified (baru)
│  ├─ iptvstore/
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
│  └─ validator/
│     └─ url.go
├─ router/
│  ├─ router.go               ← orchestrate only, panggil sub-router
│  ├─ auth.go                 ← route /auth + /admin (baru)
│  ├─ content.go              ← route /content/* (baru)
│  ├─ convert.go              ← route /convert/* (baru)
│  ├─ iptv.go                 ← route /iptv/* (baru)
│  └─ vidhub.go               ← route /vidhub/* (baru)
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
| GET | `/auth/verify` | Verifikasi API key + access token |
| GET | `/auth/quota` | Cek sisa quota API key |

### Admin (gunakan Master Key)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/admin/keys` | Buat API key baru |
| DELETE | `/admin/keys/:key` | Hapus API key |
| GET | `/admin/keys` | List semua API key |
| POST | `/admin/keys/:key/topup` | Top-up quota |

### IPTV (butuh API Key + Access Token)

#### `GET /iptv/channels`
Mengambil daftar channel. Semua query params opsional.

| Query Param | Tipe | Default | Keterangan |
|---|---|---|---|
| `country` | string | — | Filter by kode negara (contoh: `ID`, `US`). Harus valid dari `/iptv/countries` |
| `category` | string | — | Filter by kategori (contoh: `news`, `sports`). Harus valid dari `/iptv/categories` |
| `streams_only` | bool | `false` | Kalau `true`, hanya tampilkan channel yang punya stream aktif |
| `page` | integer | — | Nomor halaman. Jika diisi, pagination aktif |
| `limit` | integer | `50` | Jumlah item per halaman. Min 1, max 100. Hanya aktif jika `page` diisi |

> Jika `page` dan `limit` tidak diisi, semua channel dikembalikan sekaligus tanpa pagination.

**Contoh request:**
```
GET /iptv/channels?country=ID&streams_only=true&page=1&limit=20
```

**Contoh response dengan pagination:**
```json
{
  "success": true,
  "services": "iptv",
  "country": "ID",
  "category": "",
  "total": 120,
  "data": [...]
  "page": 1,
  "limit": 20,
  "total_pages": 6,
}
```

---

#### `GET /iptv/categories`
Mengambil seluruh daftar kategori yang tersedia. Tidak ada query params.

#### `GET /iptv/countries`
Mengambil seluruh daftar negara yang tersedia. Tidak ada query params.

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

---

## Rate Limiting

Rate limit diterapkan per endpoint group via middleware `RateLimit`:

| Group | Limit |
|---|---|
| `/content/*` | 10 req/menit per API key |
| `/convert/*` | 20 req/menit per API key |
| `/vidhub/*` | 30 req/menit per API key |
| `/iptv/*` | 60 req/menit per API key |

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
| `router/router.go` | Inisialisasi providers, proxy client, panggil sub-router |
| `router/auth.go` | Route `/auth/*` dan `/admin/*` |
| `router/content.go` | Route `/content/*` dan inisialisasi content handler |
| `router/vidhub.go` | Route `/vidhub/*` dan inisialisasi vidhub handler |
| `router/convert.go` | Route `/convert/*` dan inisialisasi convert handler |
| `router/iptv.go` | Route `/iptv/*` dan inisialisasi iptv handler |

**Aturan:** kalau menambah platform atau provider baru, `router/router.go` dan
`main.go` **tidak perlu disentuh** — cukup file sub-router yang relevan.

---

## Shared Utilities

Dua package di `pkg/` yang wajib dipakai di semua handler dan service baru:

### `pkg/httputil` — JSON Response
```go
import "vidbot-api/pkg/httputil"

// di handler, ganti c.JSON() atau writeJSONUnescaped() dengan:
httputil.WriteJSONOK(c, res)          // status 200
httputil.WriteJSON(c, http.StatusOK, res) // status custom
```
Mencegah `\u0026` pada URL di dalam response JSON. Semua handler wajib
menggunakan ini, bukan `c.JSON()` langsung untuk response yang mengandung URL.

### `pkg/fileutil` — Sanitize Filename
```go
import "vidbot-api/pkg/fileutil"

// untuk nama file tanpa ekstensi (vidhub/content service)
filename := fileutil.Sanitize(title) + ".mp4"

// untuk nama file dengan ekstensi (stream handler)
filename := fileutil.SanitizeWithExt(rawName, ext)
```

---

## Konvensi Kode

### Menambah platform content baru (misal: YouTube)

File yang perlu disentuh — tidak ada file lain:

```
1. Buat folder baru:
   internal/services/content/youtube/
   ├─ handler.go   ← ikuti pola spotify/handler.go
   └─ service.go   ← ikuti pola spotify/service.go

2. pkg/downloader/cache.go
   → tambah entry TTL di cacheTTL map

3. router/content.go
   → tambah provider slice, handler, dan route

4. cmd/seed/main.go
   → tambah allowed_domains + content:provider key

5. config/allowed_domains.json
   → tambah domain list
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
   → tambah handler dan route

4. cmd/seed/main.go
   → tambah allowed_domains key

5. config/allowed_domains.json
   → tambah domain list
```

### Menambah format convert baru

```
1. Tambah di allowedFormats map di service.go kategori yang relevan
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
| Platform content baru | `content/{nama}/` + `cache.go` + `router/content.go` + `seed` + `allowed_domains.json` |
| Provider content baru | `content/provider/{nama}/` + `router/content.go` + `seed` |
| Platform vidhub baru | `vidhub/{nama}/` + `cache.go` + `router/vidhub.go` + `seed` + `allowed_domains.json` |
| Format convert baru | `service.go` + `validator.go` + `cloudconvert.go` + `convertio.go` |
| Provider convert baru | `convert/provider/{nama}/` + `router/convert.go` |

### Response
- Selalu gunakan `httputil.WriteJSONOK` — jangan `c.JSON()` untuk response yang mengandung URL
- Error response selalu via `response.ErrorWithCode(c, status, "CODE", "message")`
- Cache selalu disimpan **tanpa** `server_1` dan `server_2`

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
| 3 | `iptvstore.startRefresh()` tidak dipanggil di `Init()` — data IPTV tidak auto-refresh | `pkg/iptvstore/store.go` | 🔴 Open |
| 4 | Goroutine secondary di content service tidak ada context cancellation | `internal/services/content/*/service.go` | 🟡 Low priority |

---

## Pending / Backlog

- [ ] Health check endpoint (`GET /health`)
- [ ] Graceful shutdown
- [ ] Structured logging ke file
- [ ] Tier sistem (free, pro, enterprise) untuk rate limit + quota berbeda
- [ ] Fix ID dan Duration kosong di response TikTok
- [ ] CF Worker: tambah Referer header untuk Convertio URLs (server_1 masih 403)
- [ ] Cache hasil convert untuk hemat credits CloudConvert/Convertio
- [ ] Dokumentasi API publik (Postman collection atau README terpisah)
- [ ] Fix `iptvstore.startRefresh()` tidak dipanggil (lihat Known Bugs #3)
- [ ] Konsolidasi `writeJSONUnescaped` ke `pkg/httputil` ✅ Selesai
- [ ] Konsolidasi `sanitizeFilename` ke `pkg/fileutil` ✅ Selesai
- [ ] Pecah `router/router.go` ke sub-router per grup ✅ Selesai

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
| Sub-router per grup | `router/router.go` tidak perlu disentuh saat tambah platform baru, tiap grup bisa dibaca independen |
| Rate limit per group via Redis | Bisa diubah tanpa redeploy, state tersimpan across instance |