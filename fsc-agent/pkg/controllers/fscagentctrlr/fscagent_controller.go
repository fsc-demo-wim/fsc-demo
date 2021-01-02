package fscagentctrlr

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiv1 "github.com/henderiw/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	fscclient "github.com/henderiw/fsc-lib-go/pkg/client/clientset/versioned"
	clfscv1 "github.com/henderiw/fsc-lib-go/pkg/client/clientset/versioned/typed/fsc.henderiw.be/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/henderiw/fsc-demo/fsc-agent/pkg/controllers/multus"

	"github.com/henderiw/fsc-demo/common/msg"

	"github.com/henderiw/fsc-demo/common/controller"
	"k8s.io/klog"
)

const workerthreads = 1
const timer = 5

// NODENAME const
const nodeNameEnv = "hypervisor3-test"

// worker Controller implements the worker controller
type workerController struct {
	ctx          context.Context
	fscClient    *fscclient.Clientset
	sriovConfig  *map[string]*string
	multusConfig *map[string]multus.NetwAtt
	Devices      *map[string]*Device
	linkCache     *map[string]map[int]*LinkAttr
	lldpLinkCache *map[string]map[int]*LldpEndPoint
	nti           clfscv1.NodeTopologyInterface
	nodeTopo      *apiv1.NodeTopology
}

// NewController returns a controller which manages work.
// ..
func NewController(ctx context.Context, fscclient *fscclient.Clientset) controller.Controller {
	mc := make(map[string]multus.NetwAtt)
	sc := make(map[string]*string)
	d := make(map[string]*Device)
	//l := make(map[string]*Link)
	lc := make(map[string]map[int]*LinkAttr)
	lldplc := make(map[string]map[int]*LldpEndPoint)
	nti := fscclient.FscV1().NodeTopologies("fsc")
	return &workerController{ctx, fscclient, &sc, &mc, &d, &lc, &lldplc, nti, nil}
}

// Run starts the controller.
func (c *workerController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {

	klog.Info("Worker controller is now running")

	for {
		var err error
		klog.Info("FSC K8s API server check, validate if node-topology already exists for this node...")
		c.nodeTopo, err = c.nti.Get(c.ctx, nodeNameEnv, metav1.GetOptions{})
		c.nodeTopo.DeepCopy()
		c.showNodeTopology()
		if err != nil {
			if strings.Contains(fmt.Sprint(err), "not found") {
				klog.Info("FSC K8s API server check, initial Node Topology not found, create it")
				c.nodeTopo.ObjectMeta.SetName(nodeNameEnv)
				c.nodeTopo.ObjectMeta.SetNamespace("fsc")
				var err2 error
				c.nodeTopo, err2 = c.nti.Create(c.ctx, c.nodeTopo, metav1.CreateOptions{})
				if err2 != nil {
					klog.Errorf("FSC K8s API server check failed, check if the fsc crds are installed: %s", err)
					time.Sleep(5 * time.Second)
				} else {
					klog.Info("FSC K8s API server check, initial node-topology created")
					break
				}
			} else if err != nil {
				klog.Errorf("FSC K8s API server check failed, check if the fsc crds are installed: %s", err)
				time.Sleep(5 * time.Second)
			}
		} else {
			break
		}
	}

	klog.Info("FSC K8s API server is functioning...")

	for {
		select {
		case m := <-workCh:
			switch m.Type {
			case msg.TimerWork:
				klog.Info("Timer based workloop kickoff")
				//for k, v := range *c.sriovConfig {
				//klog.Infof("SRIOV timer info: resourceName: %s, interfaceName: %s", k, *v)
				//}
				for k, v := range *c.multusConfig {
					if v.Kind == "sriov" {
						if newItfceName, ok := (*c.sriovConfig)[v.ResourceName]; ok {
							(*c.multusConfig)[k] = multus.NetwAtt{
								Kind:         v.Kind,
								ItfceName:    *newItfceName,
								ResourceName: v.ResourceName,
								Vlan:         v.Vlan,
								Ns:           v.Ns,
							}
						}
					}
					klog.Infof("MULTUS timer info: multusName: %s, netAttach: %v", k, v)
				}

				// revalidate the link cache
				klog.Info("revalidate LinkCache")
				if err := c.UpdateLinkCache(); err != nil {
					klog.Error(err)
				}
				if err := c.ValidateLinkCacheChanges(); err != nil {
					klog.Error(err)
				}

				// revalidate the LLDP link cache
				klog.Info("revalidate LLDP LinkCache")
				if err := c.UpdateLldpLinkCache(); err != nil {
					klog.Error(err)
				}
				if err := c.ValidateLldpLinkCacheChanges(); err != nil {
					klog.Error(err)
				}

				// provide a device centric view
				klog.Info("revalidate a device centric view")
				if err := c.DeviceCentricView(); err != nil {
					klog.Error(err)
				}
				changed, err := c.ValidateDeviceChanges()
				if err != nil {

					klog.Error(err)
				}

				if changed {
					klog.Info("Updated K8sFSC NodeTopology API")
					if err := c.UpdateK8sNodeTopology(); err != nil {
						klog.Error(err)
					}
				} else {
					klog.Info("No Update required for the K8sFSC NodeTopology API")
				}

				// check node topology
				if c.nodeTopo == nil {
					klog.Info("NodeTopology does not exist")
				} else {
					klog.Infof("NodeTopology exists: %v", *c.nodeTopo)
					c.showNodeTopology()
				}

				break
			case msg.SriovUpdate:
				switch x := m.KeyValue.(type) {
				case map[string]*string:
					*c.sriovConfig = x
				}
				break
			case msg.SriovDelete:
				switch x := m.KeyValue.(type) {
				case map[string]*string:
					*c.sriovConfig = x
				}
				break
			case msg.MultusUpdate:
				switch x := m.KeyValue.(type) {
				case map[string]multus.NetwAtt:
					for k, v := range x {
						(*c.multusConfig)[k] = v
					}
				}
				break
			case msg.MultusDelete:
				switch x := m.KeyValue.(type) {
				case map[string]multus.NetwAtt:
					for k := range x {
						delete(*c.multusConfig, k)
					}
				}
				break
			default:
				klog.Info("Wrong message received")
			}
		case <-stopCh:
			klog.Info("Stopping timeout controller")
			return
		}
	}

}
