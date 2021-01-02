package fscctrlctrlr

import (
	"context"
	"fmt"
	"reflect"

	apiv1 "github.com/henderiw/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	fscclient "github.com/henderiw/fsc-lib-go/pkg/client/clientset/versioned"
	clfscv1 "github.com/henderiw/fsc-lib-go/pkg/client/clientset/versioned/typed/fsc.henderiw.be/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/henderiw/fsc-demo/common/msg"

	"github.com/henderiw/fsc-demo/common/controller"
	"k8s.io/klog"
)

const workerthreads = 1
const timer = 5

// worker Controller implements the worker controller
type workerController struct {
	ctx            context.Context
	fscClient      *fscclient.Clientset
	Node           map[string]*v1.Node
	NodeTopology   map[string]*apiv1.NodeTopology
	WorkLoad       map[string]*apiv1.WorkLoad
	WorkLoadStatus map[string]*apiv1.WorkLoadStatus
	wli            clfscv1.WorkLoadInterface
}

// NewController returns a controller which manages work.
// ..
func NewController(ctx context.Context, fscclient *fscclient.Clientset) controller.Controller {
	n := make(map[string]*v1.Node)
	nt := make(map[string]*apiv1.NodeTopology)
	w := make(map[string]*apiv1.WorkLoad)
	wlStatus := make(map[string]*apiv1.WorkLoadStatus)
	wli := fscclient.FscV1().WorkLoads("fsc")

	return &workerController{ctx, fscclient, n, nt, w, wlStatus, wli}
}

// Run starts the controller.
func (c *workerController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {

	klog.Info("FSC Controller controller is now running")

	// TODO connect to FSP

	for {
		select {
		case m := <-workCh:
			switch m.Type {
			case msg.TimerWork:
				klog.Info("Timer based workloop kickoff")

				for k := range c.Node {
					fmt.Printf("Node: %s, Node Attributes \n", k)
				}

				for k := range c.NodeTopology {
					fmt.Printf("NodeTopology: %s, NodeTopology Attributes \n", k)
				}

				for k := range c.WorkLoad {
					fmt.Printf("Workload: %s, Workload Attributes \n", k)
				}

				newWorkloadStatus := c.constructWorkloadUpdates()
				c.showWorkLoads()
				c.compareWorkloadStatus(newWorkloadStatus)

				break
			case msg.NodeUpdate:
				klog.Infof("NodeUpdate message received %v", reflect.TypeOf(m.KeyValue))
				switch x := m.KeyValue.(type) {
				case map[string]*v1.Node:
					for k, v := range x {
						c.Node[k] = v
					}
				}
				break
			case msg.NodeDelete:
				klog.Infof("NodeDelete message received %v", reflect.TypeOf(m.KeyValue))
				switch x := m.KeyValue.(type) {
				case map[string]*v1.Node:
					for k := range x {
						delete(c.Node, k)
					}
				}
				break
			case msg.NodeTopologyUpdate:
				klog.Infof("NodeTopologyUpdate message received %v", reflect.TypeOf(m.KeyValue))
				switch x := m.KeyValue.(type) {
				case map[string]*apiv1.NodeTopology:
					for k, v := range x {
						c.NodeTopology[k] = v
					}
				}
				break
			case msg.NodeTopologyDelete:
				klog.Infof("NodeTopologyDelete message received %v", reflect.TypeOf(m.KeyValue))
				switch x := m.KeyValue.(type) {
				case map[string]*apiv1.NodeTopology:
					for k := range x {
						delete(c.NodeTopology, k)
					}
				}
				break
			case msg.WorkloadUpdate:
				klog.Infof("WorkloadUpdate message received %v", reflect.TypeOf(m.KeyValue))
				switch x := m.KeyValue.(type) {
				case map[string]*apiv1.WorkLoad:
					for k, v := range x {
						c.WorkLoad[k] = v
					}
				}
				break
			case msg.WorkloadDelete:

				klog.Infof("WorkloadDelete message received %v", reflect.TypeOf(m.KeyValue))
				switch x := m.KeyValue.(type) {
				case map[string]*apiv1.WorkLoad:
					for k := range x {
						delete(c.WorkLoad, k)
					}
					// TODO delete the workload from FSP
				}
				break
			default:
				klog.Info("Wrong message received")
			}
		case <-stopCh:
			klog.Info("Stopping FSC Controller controller")
			return
		}
	}

}
