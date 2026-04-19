package response

import (
	"fmt"
	"log/slog"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

// ─── End user domain helpers ──────────────────────────────────────────────────
// Setiap fungsi menggabungkan tiga hal sekaligus: slog + stats.TrackError + Write.
// Dipakai di handler end user: /content, /vidhub, /convert, /app, /leakcheck.
//
// Tujuan: handler tidak perlu ingat urutan log → track → write,
// cukup satu baris panggilan.

// Extraction dipanggil ketika proses extract URL gagal.
// Log: slog.Error | Track: EXTRACTION_FAILED | Response: ErrExtractionFailed
//
// Contoh: response.Extraction(c, "vidhub", "videb", err)
func Extraction(c *gin.Context, group, platform string, err error) {
	slog.Error("extract failed",
		"group", group,
		"platform", platform,
		"error", err,
	)
	stats.TrackError(c, group, platform, ErrExtractionFailed.Code)
	Write(c, ErrExtractionFailed)
}

// Convert dipanggil ketika konversi gagal di sisi provider.
// Log: slog.Error | Track: CONVERT_FAILED | Response: ErrConvertFailed
//
// Contoh: response.Convert(c, "audio", err)
func Convert(c *gin.Context, category string, err error) {
	slog.Error("provider conversion failed",
		"group", "convert",
		"category", category,
		"error", err,
	)
	stats.TrackError(c, "convert", category, ErrConvertFailed.Code)
	Write(c, ErrConvertFailed)
}

// ConvertErr dipanggil ketika submit konversi gagal sebelum sampai ke provider.
// Biasanya karena URL tidak accessible atau format tidak support.
// Log: slog.Error | Track: CONVERT_ERROR | Response: ErrConvertError
//
// Contoh: response.ConvertErr(c, "audio", err)
func ConvertErr(c *gin.Context, category string, err error) {
	slog.Error("conversion submit failed",
		"group", "convert",
		"category", category,
		"error", err,
	)
	stats.TrackError(c, "convert", category, ErrConvertError.Code)
	Write(c, ErrConvertError)
}

// DB dipanggil ketika query database gagal di handler end user.
// Log: slog.Error | Track: DB_ERROR | Response: ErrDBError
//
// Contoh: response.DB(c, "app", "android", err)
func DB(c *gin.Context, group, platform string, err error) {
	slog.Error("db query failed",
		"group", group,
		"platform", platform,
		"error", err,
	)
	stats.TrackError(c, group, platform, ErrDBError.Code)
	Write(c, ErrDBError)
}

// InvalidURLWarn dipanggil ketika URL tidak valid atau domain tidak diizinkan.
// Pakai slog.Warn (bukan Error) karena ini kesalahan client, bukan server.
// Tidak di-track ke stats karena bukan error server.
//
// Contoh: response.InvalidURLWarn(c, "vidhub", "videb", req.URL)
func InvalidURLWarn(c *gin.Context, group, platform, url string) {
	slog.Warn("invalid or disallowed url attempt",
		"group", group,
		"platform", platform,
		"url", url,
	)
	Write(c, ErrInvalidURL)
}

// ─── Admin domain helpers ─────────────────────────────────────────────────────
// Dipakai di handler admin: /admin/*.
// Pesan boleh eksplisit karena hanya admin yang bisa akses endpoint ini.
// Tidak perlu stats.TrackError — admin action tidak perlu di-track ke stats user.

// AdminDB dipanggil ketika operasi database gagal di handler admin.
// Log: slog.Error | Response: ErrAdminDBError
//
// Contoh: response.AdminDB(c, "save key", err)
func AdminDB(c *gin.Context, operation string, err error) {
	slog.Error("admin db operation failed",
		"operation", operation,
		"error", err,
	)
	Write(c, ErrAdminDBError)
}

// AdminNotFound dipanggil ketika resource tidak ditemukan di handler admin.
// Pesan eksplisit karena admin perlu tahu resource apa yang tidak ada.
//
// Contoh: response.AdminNotFound(c, "API key tidak ditemukan.")
func AdminNotFound(c *gin.Context, msg string) {
	WriteMsg(c, ErrAdminNotFound, msg)
}

// AdminBadRequest dipanggil untuk validasi input di handler admin.
// Pesan eksplisit karena admin perlu tahu field mana yang salah.
//
// Contoh: response.AdminBadRequest(c, "name, email, dan quota wajib diisi.")
func AdminBadRequest(c *gin.Context, msg string) {
	WriteMsg(c, ErrAdminBadRequest, msg)
}

// AdminServiceError dipanggil ketika terjadi error internal di handler admin
// yang bukan db error — misalnya gagal generate token, gagal marshal JSON, dll.
// Log: slog.Error | Response: ErrAdminServiceError
//
// Contoh: response.AdminServiceError(c, "generate session token", err)
func AdminServiceError(c *gin.Context, operation string, err error) {
	slog.Error("admin internal error",
		"operation", operation,
		"error", err,
	)
	Write(c, ErrAdminServiceError)
}

// AdminInvalidGroup dipanggil ketika group tidak dikenali di feature flag handler.
//
// Contoh: response.AdminInvalidGroup(c, "unknown-group")
func AdminInvalidGroup(c *gin.Context, group string) {
	WriteMsg(c, ErrAdminInvalidGroup,
		fmt.Sprintf("Group '%s' is not recognized.", group))
}

// AdminInvalidPlatform dipanggil ketika platform tidak valid untuk group tertentu.
//
// Contoh: response.AdminInvalidPlatform(c, "videb", "convert")
func AdminInvalidPlatform(c *gin.Context, platform, group string) {
	WriteMsg(c, ErrAdminInvalidPlatform,
		fmt.Sprintf("Platform '%s' is not valid for group '%s'.", platform, group))
}
