package cmd

import (
	"github.com/henderiw/fsc-demo/fsc-controller/pkg/fscctrlr"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

// deployCmd represents the deploy command
var initCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the fsc-controller",
	Aliases:      []string{"s"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		klog.Info("start fsc-agent ...")
		opts := []fscctrlr.Option{
			fscctrlr.WithDebug(debug),
			fscctrlr.WithTimeout(timeout),
			fscctrlr.WithConfigFile(kubeconfig),
		}

		fa, err := fscctrlr.New(opts...)
		if err != nil {
			klog.Fatalf("Cannot initialize fsc Controller %s", err)
		}

		klog.Info("initialize the controllers...")
		fa.InitControllers()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
