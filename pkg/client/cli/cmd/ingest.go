package cmd

import (
	"github.com/spf13/cobra"

	"github.com/telepresenceio/telepresence/v2/pkg/client/cli/ann"
)

type ingestCommand struct {
	workload   string
	container  string
	mountPoint string
}

func ingestCmd() *cobra.Command {
	ic := &ingestCommand{}
	cmd := &cobra.Command{
		Use:   "ingest [flags] <workload> [-- <command with arguments...>]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Ingest a container",
		Annotations: map[string]string{
			ann.Session:           ann.Required,
			ann.UpdateCheckFormat: ann.Tel2,
		},
		SilenceUsage:      true,
		SilenceErrors:     true,
		RunE:              ic.Run,
		ValidArgsFunction: ic.ValidArgs,
	}
	ic.AddFlags(cmd)
	return cmd
}

func (ic *ingestCommand) AddFlags(cmd *cobra.Command) {

}

func (ic *ingestCommand) Run(cmd *cobra.Command, args []string) error {
	return nil
}

func (ic *ingestCommand) ValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
