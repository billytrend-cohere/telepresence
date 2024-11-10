package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/telepresenceio/telepresence/rpc/v2/connector"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/ann"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/connect"
	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/daemon"
)

func leaveIngest() *cobra.Command {
	var containerName string
	cmd := &cobra.Command{
		Use:  "leave-ingest [flags] <workload_name>",
		Args: cobra.ExactArgs(1),

		Short: "Remove existing ingest",
		Annotations: map[string]string{
			ann.Session: ann.Required,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := connect.InitCommand(cmd); err != nil {
				return err
			}
			return removeIngest(cmd.Context(), strings.TrimSpace(args[0]), containerName)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			shellCompDir := cobra.ShellCompDirectiveNoFileComp
			if len(args) != 0 {
				return nil, shellCompDir
			}
			if err := connect.InitCommand(cmd); err != nil {
				return nil, shellCompDir | cobra.ShellCompDirectiveError
			}
			ctx := cmd.Context()
			userD := daemon.GetUserClient(ctx)
			resp, err := userD.List(ctx, &connector.ListRequest{Filter: connector.ListRequest_INGESTS})
			if err != nil {
				return nil, shellCompDir | cobra.ShellCompDirectiveError
			}
			if len(resp.Workloads) == 0 {
				return nil, shellCompDir
			}

			var completions []string
			for _, intercept := range resp.Workloads {
				for _, ii := range intercept.InterceptInfos {
					name := ii.Spec.Name
					if strings.HasPrefix(name, toComplete) {
						completions = append(completions, name)
					}
				}
			}
			return completions, shellCompDir
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&containerName, "container", "c", "", "Name of ingested container")
	return cmd
}

func removeIngest(ctx context.Context, workloadName, containerName string) error {
	_, err := daemon.GetUserClient(ctx).LeaveIngest(ctx, &connector.IngestIdentifier{
		WorkloadName:  workloadName,
		ContainerName: containerName,
	})
	return err
}
