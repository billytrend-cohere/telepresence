package docker

import (
	"context"
	"strings"

	rpc "github.com/telepresenceio/telepresence/rpc/v2/connector"
	"github.com/telepresenceio/telepresence/v2/pkg/client"
)

type Mount struct {
	LocalDir  string   `json:"local_dir,omitempty"     yaml:"local_dir,omitempty"`
	RemoteDir string   `json:"remote_dir,omitempty"    yaml:"remote_dir,omitempty"`
	Error     string   `json:"error,omitempty"         yaml:"error,omitempty"`
	PodIP     string   `json:"pod_ip,omitempty"        yaml:"pod_ip,omitempty"`
	Port      int32    `json:"port,omitempty"          yaml:"port,omitempty"`
	Mounts    []string `json:"mounts,omitempty"        yaml:"mounts,omitempty"`
}

func NewMount(ctx context.Context, ii *rpc.IngestInfo, mountError error) *Mount {
	if mountError != nil {
		return &Mount{Error: mountError.Error()}
	}
	if ii.MountPoint != "" {
		var port int32
		if client.GetConfig(ctx).Intercept().UseFtp {
			port = ii.FtpPort
		} else {
			port = ii.SftpPort
		}
		var mounts []string
		if tpMounts := ii.Environment["TELEPRESENCE_MOUNTS"]; tpMounts != "" {
			// This is a Unix path, so we cannot use filepath.SplitList
			mounts = strings.Split(tpMounts, ":")
		}
		return &Mount{
			LocalDir:  ii.ClientMountPoint,
			RemoteDir: ii.MountPoint,
			PodIP:     ii.PodIp,
			Port:      port,
			Mounts:    mounts,
		}
	}
	return nil
}
