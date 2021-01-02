package cmd

import (
	"time"

	"github.com/henderiw/fsc-demo/fsc-agent/pkg/fscagent"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

// deployCmd represents the deploy command
var initCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the fsc-agent",
	Aliases:      []string{"s"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		klog.Info("start fsc-agent ...")
		opts := []fscagent.Option{
			fscagent.WithDebug(debug),
			fscagent.WithTimeout(timeout),
			fscagent.WithConfigFile(kubeconfig),
		}

		fa, err := fscagent.New(opts...)
		if err != nil {
			klog.Fatalf("Cannot initialize fsc Agent %s", err)
		}

		for {
			if err := fa.CheckLldpDaemon(); err != nil {
				klog.Errorf("LLDP not setup; %s execute: 'sudo apt-get install lldpd;sudo systemctl restart lldpd' ", err)
				time.Sleep(5 * time.Second)
			} else {
				break
			}
		}

		klog.Info("initialize the controllers...")
		fa.InitControllers()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
