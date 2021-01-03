package cmd

import (
	"github.com/fsc-demo-wim/fsc-demo/fsc-controller/pkg/fscctrlr"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var initCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the fsc-controller",
	Aliases:      []string{"s"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("start fsc-controller ...")
		opts := []fscctrlr.Option{
			fscctrlr.WithDebug(debug),
			fscctrlr.WithTimeout(timeout),
			fscctrlr.WithConfigFile(kubeconfig),
		}

		fa, err := fscctrlr.New(opts...)
		if err != nil {
			log.Fatalf("Cannot initialize fsc Controller %s", err)
		}

		log.Info("initialize the controllers...")
		fa.InitControllers()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
