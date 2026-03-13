package cloudconvert

import (
	"fmt"
	"strings"
	"vidbot-api/internal/services/convert/provider"
	cc "vidbot-api/pkg/cloudconvert"
)

const prefix = "cc_"

var supportedFormats = []string{
	// audio
	"aac", "flac", "m4a", "mp3", "ogg", "opus", "wav", "wma", "amr", "ac3",
	// document
	"csv", "doc", "docm", "docx", "dotx", "html", "md", "odt", "pdf", "ppt", "pptx", "rtf", "txt", "wps", "xls", "xlsx",
	// image
	"avif", "bmp", "eps", "gif", "heic", "heif", "ico", "jfif", "jpg", "jpeg", "mrw", "png", "psd", "raf", "tiff", "webp", "svg", "raw",
	// fonts
	"eot", "otf", "ttf", "woff", "woff2",
}

type CloudConvertProvider struct {
	client *cc.Client
}

func New(apiKey string) *CloudConvertProvider {
	return &CloudConvertProvider{client: cc.NewClient(apiKey)}
}

func (p *CloudConvertProvider) Name() string {
	return "cloudconvert"
}

func (p *CloudConvertProvider) SupportedFormats() []string {
	return supportedFormats
}

func (p *CloudConvertProvider) Submit(fileURL, toFormat string) (string, error) {
	job, err := p.client.CreateJob(cc.JobRequest{
		Tag: "vidbot-convert",
		Tasks: map[string]cc.Task{
			"import-file": {
				Operation: "import/url",
				URL:       fileURL,
			},
			"convert-file": {
				Operation:    "convert",
				Input:        "import-file",
				OutputFormat: toFormat,
			},
			"export-file": {
				Operation: "export/url",
				Input:     "convert-file",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("cloudconvert submit failed: %w", err)
	}
	return prefix + job.ID, nil
}

func (p *CloudConvertProvider) SubmitUpload(fileData []byte, filename, toFormat string) (string, error) {
	job, err := p.client.CreateUploadJob(toFormat, "vidbot-convert-upload")
	if err != nil {
		return "", fmt.Errorf("create upload job failed: %w", err)
	}

	// cari task ID untuk import-file
	var importTaskID string
	for _, task := range job.Tasks {
		if task.Name == "import-file" {
			importTaskID = task.ID
			break
		}
	}
	if importTaskID == "" {
		return "", fmt.Errorf("import task not found")
	}

	// fetch task detail untuk dapat upload URL
	taskDetail, err := p.client.GetTask(importTaskID)
	if err != nil {
		return "", fmt.Errorf("get task detail failed: %w", err)
	}

	if taskDetail.Result == nil || taskDetail.Result.Form.URL == "" {
		return "", fmt.Errorf("upload form URL not found")
	}

	uploadURL := taskDetail.Result.Form.URL
	parameters := taskDetail.Result.Form.Parameters

	if err := p.client.UploadFile(uploadURL, parameters, filename, fileData); err != nil {
		return "", fmt.Errorf("file upload failed: %w", err)
	}

	return prefix + job.ID, nil
}

func (p *CloudConvertProvider) Status(jobID string) (*provider.ConvertResult, error) {
	rawID := strings.TrimPrefix(jobID, prefix)

	job, err := p.client.GetJob(rawID)
	if err != nil {
		return nil, err
	}

	result := &provider.ConvertResult{
		JobID:    jobID,
		Status:   job.Status,
		Provider: "cloudconvert",
	}

	if job.Status == "finished" {
		for _, task := range job.Tasks {
			if task.Name == "export-file" && task.Result != nil && len(task.Result.Files) > 0 {
				result.DownloadURL = task.Result.Files[0].URL
				result.Filename = task.Result.Files[0].Filename
				result.Size = task.Result.Files[0].Size
			}
		}
	}

	if job.Status == "error" {
		for _, task := range job.Tasks {
			if task.Status == "error" {
				result.Message = task.Message
				break
			}
		}
	}

	return result, nil
}
