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
	ctx              context.Context
	cancel           context.CancelFunc
	localMountPoint  string
	localMountPort   int32
	localPorts       []string
	handlerContainer string
	pid              int
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
	ii := &rpc.IngestInfo{
		Workload:         ig.workload,
		WorkloadKind:     ig.Kind,
		Container:        ig.container,
		PodIp:            ig.PodIp,
		SftpPort:         ig.SftpPort,
		FtpPort:          ig.FtpPort,
		MountPoint:       cn.MountPoint,
		ClientMountPoint: ig.localMountPoint,
		Environment:      cn.Environment,
	}
	if ig.handlerContainer != "" {
		if ii.Environment == nil {
			ii.Environment = make(map[string]string, 1)
		}
		ii.Environment["TELEPRESENCE_HANDLER_CONTAINER_NAME"] = ig.handlerContainer
	}
	return ii
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
	created := false
	ig, _ := s.currentIngests.Compute(ik, func(oldValue *ingest, loaded bool) (*ingest, bool) {
		if loaded {
			return oldValue, false
		}
		if ai == nil {
			ai, err = s.managerClient.EnsureAgent(ctx, &manager.EnsureAgentRequest{Session: s.sessionInfo, Name: id.WorkloadName})
			if err != nil {
				return nil, true
			}
		}
		_, ok := ai.Containers[id.ContainerName]
		if !ok {
			err = fmt.Errorf("workload %s has no container named %s", id.WorkloadName, id.ContainerName)
			return nil, true
		}
		err = s.ensureNoMountConflict(rq.MountPoint, rq.LocalMountPort)
		if err != nil {
			return nil, true
		}
		ctx, cancel := context.WithCancel(ctx)
		cancelIngest := func() {
			s.currentIngests.Delete(ik)
			dlog.Debugf(ctx, "Cancelling ingest %s", ik)
			cancel()
			s.ingestTracker.cancelContainer(ik.workload, ik.container)
		}
		created = true
		return &ingest{
			ingestKey:       ik,
			AgentInfo:       ai,
			ctx:             ctx,
			cancel:          cancelIngest,
			localMountPoint: rq.MountPoint,
			localMountPort:  rq.LocalMountPort,
			localPorts:      rq.LocalPorts,
		}, false
	})
	if err != nil {
		return nil, err
	}
	if created {
		s.ingestTracker.initialStart(ig.podAccess(s.rootDaemon))
	}
	return ig.response(), nil
}

func (s *session) ensureNoMountConflict(localMountPoint string, localMountPort int32) (err error) {
	if localMountPoint == "" && localMountPort == 0 {
		return nil
	}
	s.currentInterceptsLock.Lock()
	for _, ic := range s.currentIntercepts {
		if localMountPoint != "" && ic.ClientMountPoint == localMountPoint {
			err = status.Error(codes.AlreadyExists, fmt.Sprintf("mount point %s already in use by intercept %s", localMountPoint, ic.Spec.Name))
			break
		}
		if localMountPort != 0 && ic.localMountPort == localMountPort {
			err = status.Error(codes.AlreadyExists, fmt.Sprintf("mount port %d already in use by intercept %s", localMountPort, ic.Spec.Name))
			break
		}
	}
	s.currentInterceptsLock.Unlock()
	if err != nil {
		return err
	}

	s.currentIngests.Range(func(key ingestKey, ig *ingest) bool {
		if localMountPoint != "" && ig.localMountPoint == localMountPoint {
			err = status.Error(codes.AlreadyExists, fmt.Sprintf("mount point %s already in use by ingest %s", localMountPoint, key))
			return false
		}
		if localMountPort != 0 && ig.localMountPort == localMountPort {
			err = status.Error(codes.AlreadyExists, fmt.Sprintf("mount port %d already in use by ingest %s", localMountPort, key))
			return false
		}
		return true
	})
	return err
}

func (s *session) getIngest(rq *rpc.IngestIdentifier) (ig *ingest, err error) {
	if rq.ContainerName == "" {
		// Valid if there's only one ingest for the given workload.
		s.currentIngests.Range(func(key ingestKey, value *ingest) bool {
			if key.workload == rq.WorkloadName {
				if rq.ContainerName != "" {
					err = status.Error(codes.NotFound, fmt.Sprintf("workload %s has multiple ingests. Please specify which one to use", rq.WorkloadName))
					return false
				}
				rq.ContainerName = key.container
			}
			return true
		})
		if err != nil {
			return nil, err
		}
		if rq.ContainerName == "" {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("no ingest found for workload %s", rq.WorkloadName))
		}
	}
	ik := ingestKey{
		workload:  rq.WorkloadName,
		container: rq.ContainerName,
	}
	if ig, ok := s.currentIngests.Load(ik); ok {
		return ig, nil
	}
	return nil, status.Error(codes.NotFound, fmt.Sprintf("ingest %s doesn't exist", ik))
}

func (s *session) GetIngest(rq *rpc.IngestIdentifier) (ii *rpc.IngestInfo, err error) {
	ig, err := s.getIngest(rq)
	if err != nil {
		return nil, err
	}
	return ig.response(), nil
}

func (s *session) LeaveIngest(c context.Context, rq *rpc.IngestIdentifier) (ii *rpc.IngestInfo, err error) {
	ig, err := s.getIngest(rq)
	if err != nil {
		return nil, err
	}
	s.stopHandler(c, fmt.Sprintf("%s/%s", ig.workload, ig.container), ig.handlerContainer, ig.pid)
	ig.cancel()
	return ig.response(), nil
}
