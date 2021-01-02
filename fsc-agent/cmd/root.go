package cmd

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var debug bool
var timeout time.Duration

// path to files
var kubeconfig string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "fsc-agent",
	Short: "fsc-agent discovers server2switch topology for SRIOV/IPVLAN/MACVLAN connectiivty in a k8s cluster",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debug {
			log.SetLevel(log.DebugLevel)
		} else {
			log.SetLevel(log.InfoLevel)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "path to kubeconfig file for dev/test outside the k8s cluster")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 5*time.Second, "timeout for discovery loops, e.g: 30s, 1m, 2m30s")

}
