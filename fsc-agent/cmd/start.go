package cmd

import (
	"time"

	"github.com/fsc-demo-wim/fsc-demo/fsc-agent/pkg/controllers/fscagentctrlr"
	"github.com/fsc-demo-wim/fsc-demo/fsc-agent/pkg/fscagent"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var initCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the fsc-agent",
	Aliases:      []string{"s"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("start fsc-agent ...")
		opts := []fscagent.Option{
			fscagent.WithDebug(debug),
			fscagent.WithTimeout(timeout),
			fscagent.WithConfigFile(kubeconfig),
		}

		fa, err := fscagent.New(opts...)
		if err != nil {
			log.Fatalf("Cannot initialize fsc Agent %s", err)
		}

	MAINLOOP:
		for {
			if err := fscagentctrlr.CheckLldpDaemon(); err != nil {
				log.Errorf("LLDP daemon not accesible: %s ", err)
				time.Sleep(60 * time.Second)
				continue MAINLOOP
			}

			if err := fscagentctrlr.CheckNetLinkDaemon(); err != nil {
				log.Errorf("NetLink daemon not accesible: %s ", err)
				time.Sleep(60 * time.Second)
				continue MAINLOOP
			}

			log.Info("initialize the controllers...")
			fa.InitControllers()

			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
