package fscagent

import (
	"context"

	"github.com/henderiw/fsc-demo/fsc-agent/pkg/controllers/configmap"
	"github.com/henderiw/fsc-demo/fsc-agent/pkg/controllers/fscagentctrlr"
	"github.com/henderiw/fsc-demo/fsc-agent/pkg/controllers/multus"
	"github.com/henderiw/fsc-demo/common/controllers/timer"

	"github.com/henderiw/fsc-demo/common/msg"

	"github.com/henderiw/fsc-demo/common/controller"

	"github.com/henderiw/fsc-demo/common/client"
	"github.com/henderiw/fsc-demo/common/status"
	"k8s.io/klog"
)

// Object for keeping track of controller states and statuses.
type controllerControl struct {
	ctx         context.Context
	controllers map[string]controller.Controller
	stopCh      chan struct{}
	workCh      chan msg.CMsg
}

// InitControllers function initializes the controllers
func (fa *FscAgent) InitControllers() error {
	// stopCh to synchronize the finalization for a graceful shutdown
	stopCh := make(chan struct{})
	defer close(stopCh)

	// workCh to trigger some work, only used by the timer controller
	workCh := make(chan msg.CMsg)
	defer close(workCh)

	// Create a context.
	ctx, cancel := context.WithCancel(context.Background())

	fa.CtrlrCtrl = &controllerControl{
		ctx:         ctx,
		controllers: make(map[string]controller.Controller),
		stopCh:      stopCh,
		workCh:      workCh,
	}

	fa.CtrlrCtrl.InitControllers(ctx, fa.Clients)

	// Run the health checks on a separate goroutine to check health of the api server
	if fa.HealthEnabled {
		// Create the status file. We will only update it if we have healthchecks enabled.
		s := status.New(status.DefaultStatusFile)
		klog.Info("Starting status report routine")
		go status.RunHealthChecks(ctx, s, fa.Clients.K8sClient)
	}

	// Run the controllers. This runs indefinitely.
	fa.CtrlrCtrl.RunControllers()

	// Shut down compaction, healthChecks, and configController
	cancel()

	return nil
}

// InitControllers function
func (cc *controllerControl) InitControllers(ctx context.Context, clients *client.Info) {

	multusController := multus.NewController(ctx, clients.NetClient)
	cc.controllers["Multus"] = multusController

	configMapController := configmap.NewController(ctx, clients.K8sClient)
	cc.controllers["ConfigMap"] = configMapController

	fscAgentController := fscagentctrlr.NewController(ctx, clients.FscClient)
	cc.controllers["Worker"] = fscAgentController

	timerController := timer.NewController(ctx)
	cc.controllers["Timer"] = timerController

}

// Runs all the controllers and blocks until we get a restart.
func (cc *controllerControl) RunControllers() {
	for kind, c := range cc.controllers {
		klog.Infof("Starting controller kind %s", kind)
		go c.Run(cc.stopCh, cc.workCh)
	}

	// Block until we are cancelled, or get a new configuration and need to restart
	select {
	case <-cc.ctx.Done():
		klog.Warning("context cancelled")
	}
	close(cc.stopCh)
}
