package mount

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	empty "google.golang.org/protobuf/types/known/emptypb"

	"github.com/datawire/dlib/dlog"
	"github.com/telepresenceio/telepresence/v2/pkg/client"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/daemon"
	"github.com/telepresenceio/telepresence/v2/pkg/errcat"
)

type Flags struct {
	LocalMountPort uint16 // --local-mount-port
	Mount          string // --mount // "true", "false", or desired mount point
	Enabled        bool
}

func (f *Flags) AddFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVar(&f.Mount, "mount", "true", ``+
		`The absolute path for the root directory where volumes will be mounted, $TELEPRESENCE_ROOT. Use "true" to `+
		`have Telepresence pick a random mount point (default). Use "false" to disable filesystem mounting entirely.`)

	flagSet.Uint16Var(&f.LocalMountPort, "local-mount-port", 0,
		`Do not mount remote directories. Instead, expose this port on localhost to an external mounter`)
}

func (f *Flags) Validate(cmd *cobra.Command) error {
	if f.LocalMountPort > 0 && client.GetConfig(cmd.Context()).Intercept().UseFtp {
		return errcat.User.New("only SFTP can be used with --local-mount-port. Client is configured to perform remote mounts using FTP")
	}
	if !cmd.Flag("mount").Changed {
		// Default is that mount is enabled and the path is unspecified
		f.Mount = "" // Get rid of the default string "true"
		f.Enabled = true
	} else if len(f.Mount) > 0 {
		doMount, err := strconv.ParseBool(f.Mount)
		if err != nil {
			// Not a boolean flag. Must be a path then
			f.Enabled = true
		} else {
			// Boolean flag, path unspecified
			f.Enabled = doMount
			f.Mount = ""
		}
	}
	return nil
}

func (f *Flags) ValidateConnected(ctx context.Context) error {
	if !f.Enabled {
		return nil
	}

	ud := daemon.GetUserClient(ctx)
	if ud.Containerized() {
		// Mounts will be facilitated by the Telemount plug-in connecting to our LocalMountPort
		if f.LocalMountPort == 0 {
			lma, err := client.FreePortsTCP(1)
			if err != nil {
				return err
			}
			f.LocalMountPort = uint16(lma[0].Port)
		}
		return nil
	}

	if err := checkCapability(ctx); err != nil {
		err = fmt.Errorf("remote volume mounts are disabled: %w", err)
		// Log a warning and disable, but continue
		f.Enabled = false
		dlog.Warning(ctx, err)
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if f.Mount, err = prepare(cwd, f.Mount); err != nil {
		return err
	}
	return nil
}

func checkCapability(ctx context.Context) error {
	r, err := daemon.GetUserClient(ctx).RemoteMountAvailability(ctx, &empty.Empty{})
	if err != nil {
		return err
	}
	return errcat.FromResult(r)
}
