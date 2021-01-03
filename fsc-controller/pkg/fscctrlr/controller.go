package fscctrlr

import (
	"context"

	"github.com/fsc-demo-wim/fsc-demo/common/controllers/timer"
	"github.com/fsc-demo-wim/fsc-demo/common/msg"
	"github.com/fsc-demo-wim/fsc-demo/fsc-controller/pkg/controllers/fscctrlctrlr"
	"github.com/fsc-demo-wim/fsc-demo/fsc-controller/pkg/controllers/node"
	"github.com/fsc-demo-wim/fsc-demo/fsc-controller/pkg/controllers/nodetopo"
	"github.com/fsc-demo-wim/fsc-demo/fsc-controller/pkg/controllers/workload"

	"github.com/fsc-demo-wim/fsc-demo/common/controller"

	"github.com/fsc-demo-wim/fsc-demo/common/client"
	"github.com/fsc-demo-wim/fsc-demo/common/status"
	log "github.com/sirupsen/logrus"
)

// Object for keeping track of controller states and statuses.
type controllerControl struct {
	ctx         context.Context
	controllers map[string]controller.Controller
	stopCh      chan struct{}
	workCh      chan msg.CMsg
}

// InitControllers function initializes the controllers
func (fa *FscCtrlr) InitControllers() error {
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
		log.Info("Starting status report routine")
		go status.RunHealthChecks(ctx, s, fa.Clients.K8sClient)
	}

	// Run the controllers. This runs indefinitely.
	fa.CtrlrCtrl.RunControllers()

	// Shut down all controllers
	cancel()

	return nil
}

// InitControllers function
func (cc *controllerControl) InitControllers(ctx context.Context, clients *client.Info) {

	nodeController := node.NewController(ctx, clients.K8sClient)
	cc.controllers["node"] = nodeController

	nodeTopologyController := nodetopo.NewController(ctx, clients.FscClient)
	cc.controllers["nodetopology"] = nodeTopologyController

	workloadController := workload.NewController(ctx, clients.FscClient)
	cc.controllers["workload"] = workloadController

	fscCtrlrController := fscctrlctrlr.NewController(ctx, clients.FscClient)
	cc.controllers["fscctrlr"] = fscCtrlrController

	timerController := timer.NewController(ctx)
	cc.controllers["timer"] = timerController

}

// Runs all the controllers and blocks until we get a restart.
func (cc *controllerControl) RunControllers() {
	for kind, c := range cc.controllers {
		log.Infof("Starting controller kind %s", kind)
		go c.Run(cc.stopCh, cc.workCh)
	}

	// Block until we are cancelled, or get a new configuration and need to restart
	select {
	case <-cc.ctx.Done():
		log.Warning("context cancelled")
	}
	close(cc.stopCh)
}
