package domain

import (
	"errors"
	"mime"
	"path/filepath"
	"strings"
)

type File struct {
	Key         string
	Name        string
	Size        int64
	ContentType string
}

var imageExtensions = map[string]struct{}{".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}}

var allowedExtensions = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {},
	".pdf": {}, ".txt": {}, ".csv": {}, ".xls": {}, ".xlsx": {},
	".doc": {}, ".docx": {}, ".zip": {},
}

var allowedContentTypes = map[string]map[string]struct{}{
	".png":  {"image/png": {}},
	".jpg":  {"image/jpeg": {}},
	".jpeg": {"image/jpeg": {}},
	".gif":  {"image/gif": {}},
	".webp": {"image/webp": {}},
	".pdf":  {"application/pdf": {}},
	".txt":  {"text/plain": {}},
	".csv":  {"text/plain": {}, "application/octet-stream": {}},
	".xls":  {"application/octet-stream": {}, "application/vnd.ms-excel": {}},
	".xlsx": {"application/zip": {}, "application/octet-stream": {}},
	".doc":  {"application/octet-stream": {}, "application/msword": {}},
	".docx": {"application/zip": {}, "application/octet-stream": {}},
	".zip":  {"application/zip": {}, "application/octet-stream": {}},
}

func ValidateImage(name string) error {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filepath.Base(name))))
	if _, ok := imageExtensions[extension]; !ok {
		return errors.New("file is not an allowed image")
	}
	return nil
}

func ContentType(extension string) string {
	if value := mime.TypeByExtension(extension); value != "" {
		return value
	}
	return "application/octet-stream"
}

func ValidateContentType(extension, contentType string) error {
	allowed, ok := allowedContentTypes[strings.ToLower(extension)]
	if !ok {
		return errors.New("file extension is not allowed")
	}
	if _, ok := allowed[strings.ToLower(strings.TrimSpace(contentType))]; !ok {
		return errors.New("file content does not match its extension")
	}
	return nil
}

func ValidateUpload(name string, size, maximum int64) (string, error) {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return "", errors.New("file name is required")
	}
	extension := strings.ToLower(filepath.Ext(name))
	if _, ok := allowedExtensions[extension]; !ok {
		return "", errors.New("file extension is not allowed")
	}
	if size <= 0 || size > maximum {
		return "", errors.New("file size is invalid")
	}
	return extension, nil
}
