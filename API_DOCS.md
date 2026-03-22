# vidbot-api — API Documentation

> Base URL: `https://your-domain.com`
> Semua request dan response menggunakan `Content-Type: application/json`

---

## Autentikasi

vidbot-api menggunakan dua lapis autentikasi:

### 1. API Key
Dikirim via header `X-API-Key`. Didapat dari admin setelah registrasi.

### 2. Access Token
Dikirim via header `X-Access-Token`. Didapat dari endpoint `/auth/verify`.
Token berlaku **5 menit** — setelah expired, request ulang ke `/auth/verify`.

### Flow Autentikasi
```
1. Punya API Key → hit /auth/verify → dapat access_token
2. Pakai keduanya di setiap request endpoint
3. Kalau dapat 401 "invalid or expired access token" → request access_token baru
```

---

## Auth

### `GET /auth/verify`
Verifikasi API key dan dapatkan access token.

**Header:**
```
X-API-Key: YOUR_API_KEY
```

**Response sukses:**
```json
{
  "success": true,
  "access_token": "a3f8b2c1d4e5..."
}
```

**Response gagal:**
```json
{
  "success": false,
  "code": "UNAUTHORIZED",
  "message": "invalid api key"
}
```

---

### `GET /auth/quota`
Cek sisa quota API key.

**Header:**
```
X-API-Key: YOUR_API_KEY
```

**Response:**
```json
{
  "success": true,
  "data": {
    "name": "John Doe",
    "email": "john@example.com",
    "quota": 1000,
    "quota_used": 342,
    "quota_remaining": 658
  }
}
```

---

## Admin
> Semua endpoint admin menggunakan header `X-Master-Key`

### `POST /admin/keys`
Buat API key baru.

**Header:**
```
X-Master-Key: YOUR_MASTER_KEY
```

**Body:**
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "quota": 1000
}
```

**Response:**
```json
{
  "success": true,
  "message": "API key created successfully.",
  "data": {
    "api_key": "ea024a1c35d80c...",
    "name": "John Doe",
    "email": "john@example.com",
    "active": true,
    "quota": 1000,
    "created_at": "2026-03-20T00:00:00Z"
  }
}
```

---

### `DELETE /admin/keys/:key`
Hapus (revoke) API key.

**Header:**
```
X-Master-Key: YOUR_MASTER_KEY
```

**Response:**
```json
{
  "success": true,
  "message": "API key for 'John Doe' has been revoked."
}
```

---

### `GET /admin/keys`
List semua API key. Support filter `?active=true` atau `?active=false`.

**Response:**
```json
{
  "success": true,
  "total": 2,
  "data": [
    {
      "key_hash": "ea024a1c...",
      "name": "John Doe",
      "email": "john@example.com",
      "active": true,
      "quota": 1000,
      "created_at": "2026-03-20T00:00:00Z"
    }
  ]
}
```

---

### `POST /admin/keys/:key/topup`
Top-up quota API key.

**Body:**
```json
{
  "amount": 500
}
```

**Response:**
```json
{
  "success": true,
  "message": "Quota top-up successful for 'John Doe'",
  "data": {
    "name": "John Doe",
    "old_quota": 1000,
    "added": 500,
    "new_quota": 1500
  }
}
```

---

### `GET /admin/keys/:key/usage`
Usage detail per API key.

**Response:**
```json
{
  "success": true,
  "data": {
    "name": "John Doe",
    "email": "john@example.com",
    "active": true,
    "quota": 1000,
    "quota_used": 342,
    "quota_remaining": 658,
    "created_at": "2026-03-20T00:00:00Z",
    "usage_per_group": {
      "content": 210,
      "convert": 80,
      "iptv": 45,
      "vidhub": 7
    }
  }
}
```

---

### `GET /admin/features`
Lihat status semua feature (on/off).

**Response:**
```json
{
  "success": true,
  "data": {
    "content": "on",
    "convert": "on",
    "iptv": "off",
    "vidhub": "on"
  }
}
```

---

### `POST /admin/features/:group/enable`
Nyalakan feature. Group: `content`, `convert`, `iptv`, `vidhub`.

**Response:**
```json
{
  "success": true,
  "message": "Feature 'iptv' has been enabled."
}
```

---

### `POST /admin/features/:group/disable`
Matikan feature untuk maintenance.

**Response:**
```json
{
  "success": true,
  "message": "Feature 'iptv' has been disabled."
}
```

**Response user saat feature off:**
```json
{
  "success": false,
  "code": "SERVICE_UNAVAILABLE",
  "message": "This service is temporarily unavailable for maintenance. Please try again later."
}
```

---

### `GET /admin/stats`
Statistik usage seluruh API.

**Response:**
```json
{
  "success": true,
  "data": {
    "total_keys": 24,
    "active_keys": 18,
    "usage": {
      "content": {
        "total_requests": 1240,
        "unique_keys": 12
      },
      "convert": {
        "total_requests": 380,
        "unique_keys": 7
      },
      "iptv": {
        "total_requests": 920,
        "unique_keys": 15
      },
      "vidhub": {
        "total_requests": 210,
        "unique_keys": 5
      }
    }
  }
}
```

---

## Content
> Header wajib: `X-API-Key` + `X-Access-Token`
> Rate limit: 10 request/menit per API key

### `POST /content/spotify`
Ekstrak audio Spotify.

**Body:**
```json
{
  "url": "https://open.spotify.com/track/xxx"
}
```

**Response:**
```json
{
  "success": true,
  "services": "content",
  "sites": "spotify",
  "type": "mp3",
  "data": {
    "url": "https://open.spotify.com/track/xxx",
    "title": "Nama Lagu",
    "author": "Nama Artist",
    "thumbnail": "https://...",
    "duration": "3:45",
    "track_id": "xxx"
  },
  "download": {
    "original": "https://...",
    "server_1": "https://...",
    "server_2": "https://..."
  }
}
```

---

### `POST /content/tiktok`
Ekstrak video/audio TikTok.

**Body:**
```json
{
  "url": "https://www.tiktok.com/@user/video/xxx"
}
```

**Response:**
```json
{
  "success": true,
  "services": "content",
  "sites": "tiktok",
  "type": "mp4",
  "data": {
    "url": "https://...",
    "author": "Nama",
    "username": "@username",
    "title": "Deskripsi video",
    "thumbnail": "https://...",
    "duration": 30.0
  },
  "download": {
    "video": [
      {
        "quality": "hd_no_watermark",
        "original": "https://...",
        "original_1": "https://...",
        "server_1": "https://...",
        "server_2": "https://..."
      },
      {
        "quality": "no_watermark",
        "original": "https://...",
        "server_1": "https://...",
        "server_2": "https://..."
      }
    ],
    "audio": {
      "original": "https://...",
      "server_1": "https://...",
      "server_2": "https://..."
    }
  }
}
```

---

### `POST /content/instagram`
Ekstrak video/audio Instagram.

**Body:**
```json
{
  "url": "https://www.instagram.com/p/xxx"
}
```

---

### `POST /content/twitter`
Ekstrak video/audio Twitter/X.

**Body:**
```json
{
  "url": "https://x.com/user/status/xxx"
}
```

---

### `POST /content/threads`
Ekstrak video/audio Threads.

**Body:**
```json
{
  "url": "https://www.threads.net/@user/post/xxx"
}
```

---

## Vidhub
> Header wajib: `X-API-Key` + `X-Access-Token`
> Rate limit: 30 request/menit per API key

### `POST /vidhub/videb`
### `POST /vidhub/vidoy`
### `POST /vidhub/vidbos`
### `POST /vidhub/vidarato`
### `POST /vidhub/vidnest`

Semua endpoint vidhub menggunakan body dan response yang sama:

**Body:**
```json
{
  "url": "https://videb.co/e/xxx"
}
```

**Response:**
```json
{
  "success": true,
  "services": "vidhub",
  "sites": "videb",
  "type": "mp4",
  "data": {
    "filecode": "xxx",
    "title": "Judul Video",
    "filename": "judul_video.mp4",
    "thumbnail": "https://...",
    "size": 102400
  },
  "download": {
    "original": "https://...",
    "server_1": "https://...",
    "server_2": "https://..."
  }
}
```

---

## Convert
> Header wajib: `X-API-Key` + `X-Access-Token`
> Rate limit: 20 request/menit per API key

### `POST /convert/audio`
Konversi audio via URL.

**Body:**
```json
{
  "url": "https://example.com/file.mp3",
  "from": "mp3",
  "to": "wav"
}
```

**Format yang didukung:** `mp3, wav, flac, aac, ogg, m4a, opus, wma, amr, ac3`

---

### `POST /convert/audio/upload`
Konversi audio via upload file.

**Form data:**
```
file: [file binary]
from: mp3
to: wav
```

---

### `POST /convert/document`
Konversi dokumen via URL.

**Body:**
```json
{
  "url": "https://example.com/file.docx",
  "from": "docx",
  "to": "pdf"
}
```

**Format yang didukung:** `pdf, docx, xlsx, pptx, txt, html, odt, rtf, md, xls, csv, ppt, wps, dotx, docm, doc`

---

### `POST /convert/document/upload`
Konversi dokumen via upload file.

---

### `POST /convert/image`
Konversi gambar via URL.

**Body:**
```json
{
  "url": "https://example.com/image.png",
  "from": "png",
  "to": "webp"
}
```

**Format yang didukung:** `jpg, jpeg, png, webp, gif, avif, bmp, ico, jfif, tiff, psd, raf, mrw, heic, heif, eps, svg, raw`

---

### `POST /convert/image/upload`
Konversi gambar via upload file.

---

### `POST /convert/fonts`
Konversi font via URL.

**Body:**
```json
{
  "url": "https://example.com/font.ttf",
  "from": "ttf",
  "to": "woff2"
}
```

**Format yang didukung:** `ttf, otf, woff, woff2, eot`

---

### `POST /convert/fonts/upload`
Konversi font via upload file.

---

### Response Convert (semua endpoint convert)

**Sukses:**
```json
{
  "success": true,
  "services": "convert",
  "category": "audio",
  "data": {
    "filename": "file.wav",
    "size": 204800,
    "provider": "cloudconvert"
  },
  "download": {
    "original": "https://...",
    "server_1": "https://...",
    "server_2": "https://..."
  }
}
```

**Masih processing (job besar):**
```json
{
  "success": true,
  "data": {
    "job_id": "cc_xxx",
    "status": "processing",
    "message": "Job is still processing. Use job_id to check status."
  }
}
```

---

### `GET /convert/status/:job_id`
Cek status job konversi.

**Response:**
```json
{
  "success": true,
  "data": {
    "job_id": "cc_xxx",
    "status": "finished",
    "download_url": "https://...",
    "filename": "file.wav",
    "size": 204800,
    "provider": "cloudconvert"
  }
}
```

---

## IPTV
> Header wajib: `X-API-Key` + `X-Access-Token`
> Rate limit: 60 request/menit per API key

### `GET /iptv/channels`
Daftar channel IPTV.

**Query params (semua opsional):**
| Param | Keterangan | Contoh |
|---|---|---|
| `country` | Filter by kode negara | `ID` |
| `category` | Filter by kategori | `news` |
| `streams_only` | Hanya channel yang punya stream | `true` |
| `page` | Nomor halaman | `1` |
| `limit` | Jumlah per halaman (max 100) | `20` |

**Response:**
```json
{
  "success": true,
  "services": "iptv",
  "country": "ID",
  "total": 143,
  "data": [
    {
      "id": "SCTV.id",
      "name": "SCTV",
      "logo": "https://...",
      "country": "ID",
      "categories": ["general"],
      "website": "https://sctv.co.id",
      "streams": [
        {
          "title": "SCTV",
          "url": "https://...",
          "format": "hls",
          "quality": "720p",
          "user_agent": "",
          "referrer": ""
        }
      ]
    }
  ]
}
```

**Format stream:** `hls`, `dash`, `smil`, `direct`

---

### `GET /iptv/categories`
Daftar kategori channel.

**Response:**
```json
{
  "success": true,
  "services": "iptv",
  "total": 30,
  "data": [
    {
      "id": "news",
      "name": "News",
      "description": "News channels"
    }
  ]
}
```

---

### `GET /iptv/countries`
Daftar negara yang tersedia.

**Response:**
```json
{
  "success": true,
  "services": "iptv",
  "total": 250,
  "data": [
    {
      "name": "Indonesia",
      "code": "ID",
      "languages": ["ind"],
      "flag": "🇮🇩"
    }
  ]
}
```

---

### `GET /iptv/playlist`
Generate file M3U playlist — langsung bisa dibuka di VLC atau Tivimate.

> Auth via query param `?key=` — tidak perlu header

**Query params:**
| Param | Keterangan | Contoh |
|---|---|---|
| `key` | API Key (wajib) | `ea024a1c...` |
| `country` | Filter by negara (opsional) | `ID` |
| `category` | Filter by kategori (opsional) | `news` |

**Contoh URL untuk VLC/Tivimate:**
```
https://your-domain.com/iptv/playlist?country=ID&key=YOUR_API_KEY
```

**Response:** File M3U (`application/x-mpegurl`)

---

## Stream

### `GET /dl`
Proxy download stream. URL di-generate otomatis oleh server di field `server_2` response content/vidhub/convert — tidak perlu dibuat manual.

---

## Error Codes

| Code | HTTP Status | Keterangan |
|---|---|---|
| `UNAUTHORIZED` | 401 | API key atau access token tidak valid |
| `QUOTA_EXCEEDED` | 429 | Quota habis |
| `RATE_LIMIT_EXCEEDED` | 429 | Terlalu banyak request |
| `INVALID_URL` | 400 | URL tidak valid atau tidak didukung |
| `EXTRACTION_FAILED` | 500 | Gagal ekstrak media |
| `CONVERT_ERROR` | 400 | Format tidak didukung atau URL tidak bisa diakses |
| `CONVERT_FAILED` | 500 | Konversi gagal di provider |
| `SERVICE_UNAVAILABLE` | 503 | Service sedang maintenance |
| `BAD_REQUEST` | 400 | Request tidak valid |
| `NOT_FOUND` | 404 | Data tidak ditemukan |