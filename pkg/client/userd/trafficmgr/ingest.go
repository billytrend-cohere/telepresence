package trafficmgr

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/datawire/dlib/dlog"
	rpc "github.com/telepresenceio/telepresence/rpc/v2/connector"
	"github.com/telepresenceio/telepresence/rpc/v2/daemon"
	"github.com/telepresenceio/telepresence/rpc/v2/manager"
)

type ingestKey struct {
	workload  string
	container string
}

func (ik ingestKey) String() string {
	return fmt.Sprintf("%s[%s]", ik.workload, ik.container)
}

type ingest struct {
	*manager.AgentInfo
	ingestKey
	ctx             context.Context
	cancel          context.CancelFunc
	mountPoint      string
	localMountPoint string
	localMountPort  int32
	localPorts      []string
}

func (ig *ingest) podAccess(rd daemon.DaemonClient) *podAccess {
	ni := ig.Containers[ig.container]
	pa := &podAccess{
		ctx:              ig.ctx,
		localPorts:       ig.localPorts,
		workload:         ig.workload,
		container:        ig.container,
		podIP:            ig.PodIp,
		sftpPort:         ig.SftpPort,
		ftpPort:          ig.FtpPort,
		mountPoint:       ni.MountPoint,
		clientMountPoint: ig.localMountPoint,
		localMountPort:   ig.localMountPort,
	}
	if err := pa.ensureAccess(ig.ctx, rd); err != nil {
		dlog.Error(ig.ctx, err)
	}
	return pa
}

func (ig *ingest) response() *rpc.IngestInfo {
	cn := ig.Containers[ig.container]
	return &rpc.IngestInfo{
		Workload:         ig.workload,
		Container:        ig.container,
		PodIp:            ig.PodIp,
		SftpPort:         ig.SftpPort,
		FtpPort:          ig.FtpPort,
		ClientMountPoint: ig.localMountPoint,
		Environment:      cn.Environment,
	}
}

func (s *session) getSingleContainerName(ai *manager.AgentInfo) (name string, err error) {
	switch len(ai.Containers) {
	case 0:
		err = status.Error(codes.Unavailable, fmt.Sprintf("traffic-manager %s have no support for ingest", s.managerVersion))
	case 1:
		for name = range ai.Containers {
		}
	default:
		err = status.Error(codes.NotFound, fmt.Sprintf("workload %s has multiple containers. Please specify which one to use", ai.Name))
	}
	return name, err
}

func (s *session) Ingest(ctx context.Context, rq *rpc.IngestRequest) (ir *rpc.IngestInfo, err error) {
	var ai *manager.AgentInfo
	id := rq.Identifier
	if id.ContainerName == "" {
		ai, err = s.managerClient.EnsureAgent(ctx, &manager.EnsureAgentRequest{Session: s.sessionInfo, Name: id.WorkloadName})
		if err != nil {
			return nil, err
		}
		id.ContainerName, err = s.getSingleContainerName(ai)
		if err != nil {
			return nil, err
		}
	}

	ik := ingestKey{
		workload:  id.WorkloadName,
		container: id.ContainerName,
	}
	ig, _ := s.currentIngests.Compute(ik, func(oldValue *ingest, loaded bool) (ig *ingest, delete bool) {
		if loaded {
			return oldValue, false
		}
		if ai == nil {
			ai, err = s.managerClient.EnsureAgent(ctx, &manager.EnsureAgentRequest{Session: s.sessionInfo, Name: id.WorkloadName})
			if err != nil {
				return nil, true
			}
		}
		ci, ok := ai.Containers[id.ContainerName]
		if !ok {
			err = fmt.Errorf("workload %s has no container named %s", id.WorkloadName, id.ContainerName)
			return nil, true
		}
		ctx, cancel := context.WithCancel(ctx)
		cancelIngest := func() {
			s.currentIngests.Delete(ik)
			dlog.Debugf(ctx, "Cancelling ingest %s", ik)
			cancel()
			s.ingestTracker.cancelContainer(ik.workload, ik.container)
		}
		ig = &ingest{
			ingestKey:       ik,
			AgentInfo:       ai,
			ctx:             ctx,
			cancel:          cancelIngest,
			mountPoint:      ci.MountPoint,
			localMountPoint: rq.MountPoint,
			localMountPort:  rq.LocalMountPort,
			localPorts:      rq.LocalPorts,
		}
		return ig, false
	})
	if err != nil {
		return nil, err
	}

	s.ingestTracker.initialStart(ig.podAccess(s.rootDaemon))
	return ig.response(), nil
}

func (s *session) LeaveIngest(rq *rpc.IngestIdentifier) (err error) {
	if rq.ContainerName == "" {
		// Valid if there's only one ingest for the given workload.
		s.currentIngests.Range(func(key ingestKey, value *ingest) bool {
			if key.workload == rq.WorkloadName {
				if rq.ContainerName != "" {
					err = status.Error(codes.NotFound, fmt.Sprintf("workload %s has multiple ingestions. Please specify which one to use", rq.WorkloadName))
					return false
				}
				rq.ContainerName = key.container
			}
			return true
		})
		if err != nil {
			return err
		}
		if rq.ContainerName == "" {
			return status.Error(codes.NotFound, fmt.Sprintf("no ingest found for workload %s", rq.WorkloadName))
		}
	}
	ik := ingestKey{
		workload:  rq.WorkloadName,
		container: rq.ContainerName,
	}
	if ig, ok := s.currentIngests.Load(ik); ok {
		ig.cancel()
		return nil
	}
	return status.Error(codes.NotFound, fmt.Sprintf("ingest %s doesn't exist", ik))
}
