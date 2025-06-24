package models

type VolumeInfo struct {
	Name       string            `json:"name"`
	MountPoint string            `json:"mount_point"`
	Driver     string            `json:"driver"`
	CreatedAt  string            `json:"created_at"`
	Labels     map[string]string `json:"labels"`
}
