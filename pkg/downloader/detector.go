package downloader

import (
	"net/url"
	"strings"
)

type VideoType string

const (
	TypeMP4     VideoType = "mp4"
	TypeMP3     VideoType = "mp3"
	TypeM3U8    VideoType = "m3u8"
	TypeHLS     VideoType = "hls"
	TypeDASH    VideoType = "dash"
	TypeTS      VideoType = "ts"
	TypeWEBM    VideoType = "webm"
	TypeFLV     VideoType = "flv"
	TypeUnknown VideoType = "unknown"
)

func DetectMediaType(downloadURL string) VideoType {
	u, err := url.Parse(downloadURL)
	if err != nil {
		return TypeUnknown
	}

	path := strings.ToLower(u.Path)

	switch {
	case strings.HasSuffix(path, ".mp4") || strings.HasSuffix(path, ".mkv") || strings.HasSuffix(path, ".avi") || strings.HasSuffix(path, ".mov"):
		return TypeMP4
	case strings.HasSuffix(path, ".mp3") || strings.HasSuffix(path, ".m4a") || strings.HasSuffix(path, ".aac") || strings.HasSuffix(path, ".ogg"):
		return TypeMP3
	case strings.HasSuffix(path, ".m3u8"):
		return TypeM3U8
	case strings.HasSuffix(path, ".ts"):
		return TypeTS
	case strings.HasSuffix(path, ".mpd"):
		return TypeDASH
	case strings.HasSuffix(path, ".webm"):
		return TypeWEBM
	case strings.HasSuffix(path, ".flv"):
		return TypeFLV
	case strings.Contains(path, "hls") || strings.Contains(path, "playlist"):
		return TypeHLS
	default:
		return TypeUnknown
	}
}

func MediaTypeToExt(t VideoType) string {
	switch t {
	case TypeMP4:
		return "mp4"
	case TypeMP3:
		return "mp3"
	case TypeM3U8, TypeHLS:
		return "m3u8"
	case TypeTS:
		return "ts"
	case TypeDASH:
		return "mpd"
	case TypeWEBM:
		return "webm"
	case TypeFLV:
		return "flv"
	default:
		return "mp4"
	}
}
