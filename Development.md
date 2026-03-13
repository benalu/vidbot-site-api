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
vidbot-api/
├─ cmd/seed/main.go           # seed Redis: allowed domains, provider priority
├─ config/
│  ├─ config.go               # load env ke struct Config
│  └─ allowed_domains.json    # domain whitelist fallback (jika Redis kosong)
├─ internal/
│  ├─ admin/handler.go        # CRUD API key, top-up quota
│  ├─ auth/handler.go         # verify token, cek quota
│  ├─ services/
│  │  ├─ content/             # ekstraksi media (spotify, tiktok, instagram)
│  │  │  ├─ provider/         # interface Provider + ResolveProviderForCategory
│  │  │  │  └─ downr/         # implementasi Downr
│  │  │  ├─ spotify/          # handler + service
│  │  │  ├─ tiktok/           # handler + service
│  │  │  └─ instagram/        # handler + service
│  │  ├─ convert/             # konversi file
│  │  │  ├─ provider/         # interface Provider + polling + resolve
│  │  │  │  ├─ cloudconvert/  # implementasi CloudConvert
│  │  │  │  └─ convertio/     # implementasi Convertio
│  │  │  ├─ audio/            # handler + service
│  │  │  ├─ document/         # handler + service
│  │  │  ├─ image/            # handler + service
│  │  │  └─ fonts/            # handler + service
│  │  └─ vidhub/              # ekstraksi dari video hosting
│  │     ├─ videb/
│  │     ├─ vidoy/
│  │     ├─ vidbos/
│  │     ├─ vidarato/
│  │     └─ vidnest/
│  └─ stream/handler.go       # proxy download stream
├─ middleware/
│  ├─ api_key.go              # validasi API key + quota
│  ├─ auth.go                 # validasi HMAC access token
│  └─ ratelimit.go            # rate limit per endpoint group
├─ pkg/
│  ├─ apikey/types.go         # struct Data API key
│  ├─ cache/cache.go          # wrapper Redis client
│  ├─ convertvalidator/       # validasi format + ukuran file konversi
│  ├─ downloader/             # cache helper + URL generator (server_1, server_2)
│  ├─ limiter/                # rate limit logic (Redis-based)
│  ├─ mediaresponse/          # struct response semua kategori
│  ├─ proxy/                  # HTTP client via CF Worker
│  ├─ response/               # helper error response
│  └─ validator/              # URL validator + domain whitelist
├─ router/router.go           # setup semua route + middleware
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

### Content (butuh API Key + Access Token)
| Method | Path | Keterangan |
|---|---|---|
| POST | `/content/spotify` | Ekstrak audio Spotify |
| POST | `/content/tiktok` | Ekstrak video/audio TikTok |
| POST | `/content/instagram` | Ekstrak video/audio Instagram |

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
content:provider:tiktok     → ["downr"]
content:provider:instagram  → ["downr"]
convert:provider:audio      → ["cloudconvert", "convertio"]
convert:provider:document   → ["cloudconvert", "convertio"]
convert:provider:image      → ["cloudconvert", "convertio"]
convert:provider:fonts      → ["cloudconvert", "convertio"]
```

### Ganti provider tanpa redeploy
```bash
# ganti urutan priority
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

Untuk mengubah limit, edit `endpointLimits` di `pkg/limiter/ratelimit.go`.

---

## Cache

Response di-cache di Redis untuk mengurangi hit ke provider eksternal.
`server_1` dan `server_2` **tidak disimpan** di cache — di-generate ulang saat cache hit.

| Key | TTL |
|---|---|
| `content:spotify` | 30 hari |
| `content:tiktok` | 10 menit |
| `content:instagram` | 10 menit |
| `vidhub:videb` | 2 jam |
| `vidhub:vidoy` | 1 jam |
| `vidhub:vidbos` | 2 jam |
| `vidhub:vidarato` | 2 jam |
| `vidhub:vidnest` | 2 jam |

---

## Konvensi Kode

### Menambah endpoint content baru (misal: youtube)
1. Buat folder `internal/services/content/youtube/`
2. Buat `service.go` — struct `Service`, method `Extract`, gunakan `provider.ResolveProviderForCategory`
3. Buat `handler.go` — struct `Handler`, method `Extract`, gunakan `writeJSONUnescaped`
4. Tambah struct response di `pkg/mediaresponse/response.go` jika struktur berbeda
5. Tambah TTL di `pkg/downloader/cache.go`
6. Tambah route di `router/router.go`
7. Tambah allowed domains + provider seed di `cmd/seed/main.go`
8. Jalankan `go run cmd/seed/main.go`

### Menambah provider baru (misal: cobalt)
1. Buat folder `internal/services/content/provider/cobalt/`
2. Implementasikan interface `Provider`: `Name()`, `Extract()`
3. Daftarkan di `router/router.go` ke slice `providers`
4. Update seed di `cmd/seed/main.go`

### Menambah format convert baru
1. Tambah di `allowedFormats` di `service.go` kategori yang relevan
2. Tambah di `formatCompatibility` map
3. Tambah content type di `pkg/convertvalidator/validator.go`
4. Tambah di `supportedFormats` di kedua provider (cloudconvert + convertio)

### Response
- Selalu gunakan `writeJSONUnescaped` untuk menghindari `\u0026` di URL
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

## Pending / Backlog

- [ ] Health check endpoint (`GET /health`)
- [ ] Graceful shutdown
- [ ] Structured logging ke file
- [ ] Tier sistem (free, pro, enterprise) untuk rate limit + quota berbeda
- [ ] Fix ID dan Duration kosong di response TikTok
- [ ] CF Worker: tambah Referer header untuk Convertio URLs (server_1 masih 403)
- [ ] Cache hasil convert untuk hemat credits CloudConvert/Convertio
- [ ] Dokumentasi API publik (Postman collection atau README terpisah)

---

## Keputusan Teknis

| Keputusan | Alasan |
|---|---|
| Provider pattern dengan Redis priority | Ganti provider tanpa redeploy |
| `from` wajib di convert | Cegah hit provider untuk kombinasi format yang tidak support, hemat credits |
| Cache tanpa server_1/server_2 | URL download berisi HMAC yang time-based, tidak bisa disimpan permanen |
| Hex encoding untuk download URL | Karakter aman, tidak ada padding atau karakter spesial |
| `writeJSONUnescaped` | Mencegah `\u0026` di URL dalam response JSON |
| Rate limit per group via Redis | Bisa diubah tanpa redeploy, state tersimpan across instance |
