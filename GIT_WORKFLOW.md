# Git Workflow — vidbot-site-api

Panduan singkat agar tidak lupa push dan tetap on-track.

---

## Rutinitas Harian

### Mulai kerja
```bash
git pull                  # ambil perubahan terbaru (kalau kerja di beberapa mesin)
```

### Selama kerja
```bash
git status                # cek file apa yang berubah
git add .                 # staging semua perubahan
git commit -m "fix: something"              # tulis commit message (hook akan muncul sebagai reminder)
```

### Selesai kerja — JANGAN LUPA
```bash
git push                  # push ke GitHub
```

---

## Konvensi Commit Message

```
feat:     fitur baru
fix:      perbaikan bug
chore:    perubahan non-fungsional (update dependency, config)
refactor: refaktor kode tanpa mengubah behavior
docs:     update dokumentasi
```

Contoh:
```
feat: tambah endpoint /content/youtube
fix: encoding URL di tiktok handler
docs: update DEVELOPMENT.md
chore: tambah format avif ke image converter
```

---

## Checklist Sebelum Push

- [ ] `git status` — tidak ada file sensitif yang ikut (`.env`, `tools/`)
- [ ] Commit message mengikuti konvensi
- [ ] Update `CHANGELOG.md` kalau ada perubahan signifikan
- [ ] Update `TODO.md` kalau ada task yang selesai atau baru

---

## Kalau Lupa Push Beberapa Hari

```bash
# cek commit yang belum dipush
git log origin/main..HEAD --oneline

# push semuanya sekaligus
git push
```

---

## Kalau Kerja di Mesin Lain / Setup Ulang

```bash
# clone repo
git clone https://github.com/benalu/vidbot-site-api.git

# masuk folder
cd vidbot-site-api

# install dependencies
go mod tidy

# copy .env.example dan isi nilainya
cp .env.example .env

# pasang git hook
cp prepare-commit-msg .git/hooks/prepare-commit-msg
chmod +x .git/hooks/prepare-commit-msg

# seed Redis
go run cmd/seed/main.go

# jalankan server
go run main.go or air
```

---

## Perintah Git yang Sering Dipakai

| Perintah | Fungsi |
|---|---|
| `git status` | Lihat file yang berubah |
| `git add .` | Staging semua perubahan |
| `git commit` | Commit dengan editor (hook aktif) |
| `git push` | Push ke GitHub |
| `git pull` | Ambil perubahan dari GitHub |
| `git log --oneline` | Lihat history commit ringkas |
| `git diff` | Lihat perubahan yang belum di-staging |
| `git stash` | Simpan perubahan sementara tanpa commit |
| `git stash pop` | Kembalikan perubahan dari stash |