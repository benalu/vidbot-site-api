package response

import "net/http"

// APIError adalah definisi tunggal sebuah error.
// Semua error yang mungkin terjadi di aplikasi didefinisikan di sini.
// Handler tidak perlu tahu HTTP status atau pesan — cukup pakai konstanta ini.
type APIError struct {
	Code       string
	HTTPStatus int
	Message    string
}

var (
	// ── 4xx Client Error — End User ───────────────────────────────────────────
	// Pesan sengaja dibuat generic agar tidak leak detail internal ke end user.

	ErrBadRequest = APIError{
		Code:       "BAD_REQUEST",
		HTTPStatus: http.StatusBadRequest,
		Message:    "Invalid request.",
	}
	ErrInvalidURL = APIError{
		Code:       "INVALID_URL",
		HTTPStatus: http.StatusBadRequest,
		Message:    "URL not supported for this endpoint.",
	}
	ErrURLUnreachable = APIError{
		Code:       "URL_UNREACHABLE",
		HTTPStatus: http.StatusBadRequest,
		Message:    "File URL is not accessible.",
	}
	ErrFileTooLarge = APIError{
		Code:       "FILE_TOO_LARGE",
		HTTPStatus: http.StatusBadRequest,
		Message:    "File exceeds maximum allowed size.",
	}
	ErrInvalidContentType = APIError{
		Code:       "INVALID_CONTENT_TYPE",
		HTTPStatus: http.StatusBadRequest,
		Message:    "Content type is not allowed for this conversion.",
	}
	ErrUnauthorized = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "Authentication required.",
	}
	ErrNotFound = APIError{
		Code:       "NOT_FOUND",
		HTTPStatus: http.StatusNotFound,
		Message:    "Resource not found.",
	}
	ErrQuotaExceeded = APIError{
		Code:       "QUOTA_EXCEEDED",
		HTTPStatus: http.StatusTooManyRequests,
		Message:    "Quota habis, silakan top-up.",
	}
	ErrRateLimitExceeded = APIError{
		Code:       "RATE_LIMIT_EXCEEDED",
		HTTPStatus: http.StatusTooManyRequests,
		Message:    "Too many requests. Please try again in a minute.",
	}
	ErrServiceUnavailable = APIError{
		Code:       "SERVICE_UNAVAILABLE",
		HTTPStatus: http.StatusServiceUnavailable,
		Message:    "This service is temporarily unavailable for maintenance. Please try again later.",
	}

	ErrAPIKeyMissing = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "API key is required. Provide it via X-API-Key header.",
	}
	ErrAPIKeyNotFound = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "API key not found.",
	}
	ErrAPIKeyInactive = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "API key is inactive. Please contact administrator.",
	}
	ErrAccessTokenMissing = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "Access token is required. Obtain it via X-Access-Token header from /auth/verify.",
	}
	ErrAccessTokenInvalid = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "Access token is invalid or expired. Re-fetch from /auth/verify.",
	}

	// ── 5xx Server Error — End User ───────────────────────────────────────────

	ErrExtractionFailed = APIError{
		Code:       "EXTRACTION_FAILED",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "Unable to process the requested URL. The content may be private, deleted, or temporarily unavailable.",
	}
	ErrConvertError = APIError{
		Code:       "CONVERT_ERROR",
		HTTPStatus: http.StatusBadRequest,
		Message:    "Conversion failed. Please check that the file format is supported and the URL is accessible.",
	}
	ErrConvertFailed = APIError{
		Code:       "CONVERT_FAILED",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "Conversion failed on the provider side. Please try again or use a different file.",
	}
	ErrServiceError = APIError{
		Code:       "SERVICE_ERROR",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "Something went wrong. Please try again later.",
	}
	ErrDBError = APIError{
		Code:       "DB_ERROR",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "Something went wrong. Please try again later.",
	}

	// ── Admin only ────────────────────────────────────────────────────────────
	// Pesan boleh eksplisit karena hanya admin yang bisa akses endpoint ini.
	// Key creation hanya bisa dilakukan oleh admin.

	ErrAdminBadRequest = APIError{
		Code:       "BAD_REQUEST",
		HTTPStatus: http.StatusBadRequest,
		Message:    "Request invalid.",
	}
	ErrAdminUnauthorized = APIError{
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "Authentication required. Use X-Master-Key or X-Admin-Session.",
	}
	ErrAdminSessionExpired = APIError{
		Code:       "SESSION_EXPIRED",
		HTTPStatus: http.StatusUnauthorized,
		Message:    "Session expired or invalid. Please login again.",
	}
	ErrAdminNotFound = APIError{
		Code:       "NOT_FOUND",
		HTTPStatus: http.StatusNotFound,
		Message:    "Resource tidak ditemukan.",
	}
	ErrAdminInvalidGroup = APIError{
		Code:       "INVALID_GROUP",
		HTTPStatus: http.StatusBadRequest,
		Message:    "Group tidak dikenali.",
	}
	ErrAdminInvalidPlatform = APIError{
		Code:       "INVALID_PLATFORM",
		HTTPStatus: http.StatusBadRequest,
		Message:    "Platform tidak valid untuk group ini.",
	}
	ErrAdminRateLimit = APIError{
		Code:       "RATE_LIMIT_EXCEEDED",
		HTTPStatus: http.StatusTooManyRequests,
		Message:    "Too many login attempts. Please try again later.",
	}
	ErrAdminDBError = APIError{
		Code:       "DB_ERROR",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "Gagal mengakses database.",
	}
	ErrAdminServiceError = APIError{
		Code:       "SERVICE_ERROR",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "Terjadi kesalahan internal.",
	}
)
