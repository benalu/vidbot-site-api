# Changelog

Format: `feat | fix | chore | refactor | docs`
Update setiap selesai satu sesi kerja — tidak perlu per commit.

---

## [Unreleased]

### feat
- Tambah endpoint `/content/instagram`
- Tambah endpoint `/content/tiktok`
- Tambah kategori convert: audio, document, image, fonts
- Tambah rate limit per endpoint group (content: 10/mnt, convert: 20/mnt, vidhub: 30/mnt)
- Tambah provider switching via Redis tanpa redeploy
- Tambah `from` wajib di semua kategori convert untuk validasi kompatibilitas format
- Tambah cache response untuk content (spotify: 30 hari, tiktok/instagram: 10 menit)

### fix
- Fix `\u0026` encoding di URL response — semua handler pakai `writeJSONUnescaped`
- Fix `convertvalidator.Audio` di document upload handler (harusnya `Document`)
- Fix duplikasi `strings.TrimPrefix` di document convert handler

### chore
- Tambah `DEVELOPMENT.md` sebagai single source of truth
- Tambah format image: avif, bmp, ico, jfif, tiff, psd, raf, mrw, heic, heif, eps, svg, raw
- Tambah format audio: wma, amr, ac3
- Tambah format document: xls, csv, ppt, wps, dotx, docm, doc

---

## Cara update

Cukup tambah entry di bagian `[Unreleased]` setelah selesai kerja.
Saat siap release/deploy, ganti `[Unreleased]` dengan tanggal:

```
## [2026-03-13]
```

Lalu buat section `[Unreleased]` baru di atas untuk sesi berikutnya.