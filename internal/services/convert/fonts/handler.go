package fonts

import (
	"io"
	"log"
	"net/http"
	"strings"
	"vidbot-api/internal/services/convert/provider"
	"vidbot-api/pkg/convertvalidator"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/httputil"
	"vidbot-api/pkg/mediaresponse"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/validator"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service              *Service
	downloadWorkerURL    string
	downloadWorkerSecret string
	workerXORKey         string
	appURL               string
	streamSecret         string
}

func NewHandler(providers []provider.Provider, downloadWorkerURL, downloadWorkerSecret, workerXORKey, appURL, streamSecret string) *Handler {
	return &Handler{
		service:              NewService(providers),
		downloadWorkerURL:    downloadWorkerURL,
		downloadWorkerSecret: downloadWorkerSecret,
		workerXORKey:         workerXORKey,
		appURL:               appURL,
		streamSecret:         streamSecret,
	}
}

type ConvertRequest struct {
	URL  string `json:"url" binding:"required"`
	To   string `json:"to" binding:"required"`
	From string `json:"from" binding:"required"`
}

func (h *Handler) Convert(c *gin.Context) {
	var req ConvertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithCode(c, 400, "BAD_REQUEST", "url, from, and to are required.")
		return
	}

	if !validator.IsValidURL(req.URL) {
		response.ErrorWithCode(c, 400, "INVALID_URL", "Invalid URL.")
		return
	}

	req.From = strings.ToLower(strings.TrimPrefix(req.From, "."))
	req.To = strings.ToLower(strings.TrimPrefix(req.To, "."))

	if verr := convertvalidator.Validate(req.URL, convertvalidator.Fonts); verr != nil {
		response.ErrorWithCode(c, 400, verr.Code, verr.Message)
		return
	}

	result, err := h.service.SubmitAndWait(req.URL, req.From, req.To)
	if err != nil {
		log.Printf("[convert] submit error: %v", err)
		response.ErrorWithCode(c, 400, "CONVERT_ERROR", "Conversion failed. Please check that the file format is supported and the URL is accessible.")
		return
	}

	if result.Status == "error" {
		response.ErrorWithCode(c, 500, "CONVERT_FAILED", "Conversion failed on the provider side. Please try again or use a different file.")
		return
	}

	if result.Status == "processing" {
		c.JSON(http.StatusAccepted, gin.H{
			"success": true,
			"data": gin.H{
				"job_id":  result.JobID,
				"status":  "processing",
				"message": result.Message,
			},
		})
		return
	}

	ext := req.To
	titleWithoutExt := strings.TrimSuffix(result.Filename, "."+ext)

	server1 := downloader.GenerateServer1URL(
		h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
		result.DownloadURL, titleWithoutExt, result.Filename, "", ext, "convert",
	)
	server2 := downloader.GenerateServer2URL(
		h.appURL, h.streamSecret,
		result.DownloadURL, titleWithoutExt, result.Filename, "", ext, "convert",
	)

	res := mediaresponse.ConvertResponse{
		Success:  true,
		Services: "convert",
		Category: "fonts",
		Data: mediaresponse.ConvertData{
			Filename: result.Filename,
			Size:     result.Size,
			Provider: result.Provider,
		},
		Download: mediaresponse.ConvertDownloadLinks{
			Original: result.DownloadURL,
			Server1:  server1,
			Server2:  server2,
		},
	}

	httputil.WriteJSONOK(c, res)
}

func (h *Handler) Upload(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		response.ErrorWithCode(c, 400, "BAD_REQUEST", "Failed to parse form data.")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.ErrorWithCode(c, 400, "BAD_REQUEST", "file is required.")
		return
	}
	defer file.Close()

	to := strings.ToLower(strings.TrimSpace(c.Request.FormValue("to")))
	if to == "" {
		response.ErrorWithCode(c, 400, "BAD_REQUEST", "to is required.")
		return
	}
	to = strings.TrimPrefix(to, ".")
	from := strings.ToLower(strings.TrimSpace(c.Request.FormValue("from")))
	from = strings.TrimPrefix(from, ".")
	if from == "" {
		response.ErrorWithCode(c, 400, "BAD_REQUEST", "from is required.")
		return
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		response.ErrorWithCode(c, 500, "READ_ERROR", "Failed to read uploaded file.")
		return
	}

	if verr := convertvalidator.ValidateUpload(fileData, header.Size, convertvalidator.Fonts); verr != nil {
		response.ErrorWithCode(c, 400, verr.Code, verr.Message)
		return
	}

	result, err := h.service.SubmitAndWaitUpload(fileData, header.Filename, from, to)
	if err != nil {
		log.Printf("[convert] upload error: %v", err)
		response.ErrorWithCode(c, 400, "CONVERT_ERROR", "Conversion failed. Please check that the file format is supported and try again.")
		return
	}

	if result.Status == "error" {
		response.ErrorWithCode(c, 500, "CONVERT_FAILED", "Conversion failed on the provider side. Please try again or use a different file.")
		return
	}

	if result.Status == "processing" {
		c.JSON(http.StatusAccepted, gin.H{
			"success": true,
			"data": gin.H{
				"job_id":  result.JobID,
				"status":  "processing",
				"message": result.Message,
			},
		})
		return
	}

	ext := to
	titleWithoutExt := strings.TrimSuffix(result.Filename, "."+ext)

	server1 := downloader.GenerateServer1URL(
		h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
		result.DownloadURL, titleWithoutExt, result.Filename, "", ext, "convert",
	)
	server2 := downloader.GenerateServer2URL(
		h.appURL, h.streamSecret,
		result.DownloadURL, titleWithoutExt, result.Filename, "", ext, "convert",
	)

	res := mediaresponse.ConvertResponse{
		Success:  true,
		Services: "convert",
		Category: "fonts",
		Data: mediaresponse.ConvertData{
			Filename: result.Filename,
			Size:     result.Size,
			Provider: result.Provider,
		},
		Download: mediaresponse.ConvertDownloadLinks{
			Original: result.DownloadURL,
			Server1:  server1,
			Server2:  server2,
		},
	}

	httputil.WriteJSONOK(c, res)
}

func (h *Handler) Status(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		response.ErrorWithCode(c, 400, "BAD_REQUEST", "job_id is required.")
		return
	}

	result, err := h.service.Status(jobID)
	if err != nil {
		response.ErrorWithCode(c, 500, "STATUS_ERROR", "Failed to get job status.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
