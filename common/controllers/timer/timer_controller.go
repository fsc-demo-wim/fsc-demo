package timer

import (
	"context"
	"time"

	"github.com/fsc-demo-wim/fsc-demo/common/msg"

	"github.com/fsc-demo-wim/fsc-demo/common/controller"
	"k8s.io/klog"
)

const workerthreads = 1
const timer = 5

// timerController implements the Controller interface for managing Kubernetes configmap
// information related to sriov
type timerController struct {
	ctx context.Context
}

// NewController returns a controller which manages configmaps objects.
// ..
func NewController(ctx context.Context) controller.Controller {

	return &timerController{ctx}
}

// Run starts the controller.
func (c *timerController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {

	klog.Info("Starting timer controller")

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(timer * time.Second)
		timeout <- true
	}()

	klog.Info("Timer controller is now running")
	for {
		select {
		case <-timeout:
			go func() {
				time.Sleep(5 * time.Second)
				timeout <- true
			}()
			klog.Infof("Timeout triggering work on workCh")
			m := msg.CMsg{
				Type:     msg.TimerWork,
				RespChan: nil,
			}
			workCh <- m
		case <-stopCh:
			klog.Info("Stopping timeout controller")
			return
		}
	}
}
