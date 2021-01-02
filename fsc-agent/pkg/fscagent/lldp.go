package fscagent

import (
	"fmt"
	"os/exec"

	"k8s.io/klog"
)

// CheckLldpDaemon function
func (fs *FscAgent) CheckLldpDaemon() error {
	cmd := exec.Command("systemctl", "check", "lldpd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("systemctl finished with non-zero: %v, status: %s", exitErr, string(out))
		}
	}
	klog.Infof("LLDP Systemctl Status is: %s", string(out))
	return nil
}
