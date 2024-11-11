package ingest

import (
	"context"
	"io"

	rpc "github.com/telepresenceio/telepresence/rpc/v2/connector"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/docker"
	"github.com/telepresenceio/telepresence/v2/pkg/ioutil"
)

type Info struct {
	Workload    string            `json:"workload,omitempty"              yaml:"workload,omitempty"`
	Container   string            `json:"container,omitempty"            yaml:"container,omitempty"`
	Environment map[string]string `json:"environment,omitempty"     yaml:"environment,omitempty"`
	Mount       *docker.Mount     `json:"mount,omitempty"           yaml:"mount,omitempty"`
	PodIP       string            `json:"pod_ip,omitempty"          yaml:"pod_ip,omitempty"`
}

func NewInfo(ctx context.Context, ii *rpc.IngestInfo, mountError error) *Info {
	return &Info{
		Workload:    ii.Workload,
		Container:   ii.Container,
		Mount:       docker.NewMount(ctx, ii, mountError),
		PodIP:       ii.PodIp,
		Environment: ii.Environment,
	}
}

func (ii *Info) WriteTo(w io.Writer) (int64, error) {
	kvf := ioutil.DefaultKeyValueFormatter()
	kvf.Prefix = "   "
	kvf.Add("Workload", ii.Workload)
	kvf.Add("Container", ii.Container)
	if m := ii.Mount; m != nil {
		if m.LocalDir != "" {
			kvf.Add("Volume Mount Point", m.LocalDir)
		} else if m.Error != "" {
			kvf.Add("Volume Mount Error", m.Error)
		}
	}
	return kvf.WriteTo(w)
}
