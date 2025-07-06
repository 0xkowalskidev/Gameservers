package models

import "fmt"

type FileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"`
}

// FormattedSize returns the size in human readable format
func (f *FileInfo) FormattedSize() string {
	const unit = 1024
	if f.Size < unit {
		return fmt.Sprintf("%d B", f.Size)
	}
	div, exp := int64(unit), 0
	for n := f.Size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(f.Size)/float64(div), "KMGTPE"[exp])
}
