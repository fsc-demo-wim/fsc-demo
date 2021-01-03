package fscagentctrlr

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	apiv1 "github.com/fsc-demo-wim/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	fscclient "github.com/fsc-demo-wim/fsc-lib-go/pkg/client/clientset/versioned"
	clfscv1 "github.com/fsc-demo-wim/fsc-lib-go/pkg/client/clientset/versioned/typed/fsc.henderiw.be/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fsc-demo-wim/fsc-demo/fsc-agent/pkg/controllers/multus"

	"github.com/fsc-demo-wim/fsc-demo/common/msg"

	"github.com/fsc-demo-wim/fsc-demo/common/controller"
	log "github.com/sirupsen/logrus"
)

const workerthreads = 1
const timer = 5

// NODENAME const
const nodeNameEnv = "NODE_NAME"

// worker Controller implements the worker controller
type workerController struct {
	ctx           context.Context
	fscClient     *fscclient.Clientset
	sriovConfig   *map[string]*string
	multusConfig  *map[string]multus.NetwAtt
	Devices       *map[string]*Device
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
	lc := make(map[string]map[int]*LinkAttr)
	lldplc := make(map[string]map[int]*LldpEndPoint)
	nti := fscclient.FscV1().NodeTopologies("fsc")
	return &workerController{ctx, fscclient, &sc, &mc, &d, &lc, &lldplc, nti, nil}
}

// Run starts the controller.
func (c *workerController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {

	log.Info("Worker controller is now running")

	// checks if the node topology api is functioning before we handle the real work
	for {
		var err error
		log.Info("FSC K8s API server check, validate if node-topology already exists for this node...")
		nodeName := os.Getenv(nodeNameEnv)
		c.nodeTopo, err = c.nti.Get(c.ctx, nodeName, metav1.GetOptions{})
		c.nodeTopo.DeepCopy()
		c.showNodeTopology()
		if err != nil {
			if strings.Contains(fmt.Sprint(err), "not found") {
				log.Info("FSC K8s API server check, initial Node Topology not found, create it")
				c.nodeTopo.ObjectMeta.SetName(nodeName)
				c.nodeTopo.ObjectMeta.SetNamespace("fsc")
				var err2 error
				c.nodeTopo, err2 = c.nti.Create(c.ctx, c.nodeTopo, metav1.CreateOptions{})
				if err2 != nil {
					log.Errorf("FSC K8s API server check failed, check if the fsc crds are installed: %s", err)
					time.Sleep(5 * time.Second)
				} else {
					log.Info("FSC K8s API server check, initial node-topology created")
					break
				}
			} else if err != nil {
				log.Errorf("FSC K8s API server check failed, check if the fsc crds are installed: %s", err)
				time.Sleep(5 * time.Second)
			}
		} else {
			break
		}
	}

	log.Info("FSC K8s API server is alive...")

	for {
		select {
		case m := <-workCh:
			switch m.Type {
			case msg.TimerWork:
				log.Info("Timer based workloop kickoff...")
				for k, v := range *c.sriovConfig {
					log.Debugf("SRIOV timer info: resourceName: %s, interfaceName: %s", k, *v)
				}
				// augment the multus configuration with sriov config
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
					log.Infof("MULTUS timer info: multusName: %s, netAttach: %v", k, v)
				}

				// revalidate the link cache
				log.Info("revalidate LinkCache...")
				if err := c.UpdateNetLinkCache(); err != nil {
					log.Error(err)
				}
				if err := c.ValidateNetLinkCacheChanges(); err != nil {
					log.Error(err)
				}

				// revalidate the LLDP link cache
				log.Info("revalidate LLDP LinkCache...")
				if err := c.UpdateLldpLinkCache(); err != nil {
					log.Error(err)
				}
				if err := c.ValidateLldpLinkCacheChanges(); err != nil {
					log.Error(err)
				}

				// provide a device centric view
				log.Info("revalidate a device centric view")
				if err := c.DeviceCentricView(); err != nil {
					log.Error(err)
				}
				changed, err := c.ValidateDeviceChanges()
				if err != nil {

					log.Error(err)
				}

				if changed {
					log.Info("Updated K8sFSC NodeTopology API")
					if err := c.UpdateK8sNodeTopology(); err != nil {
						log.Error(err)
					}
				} else {
					log.Info("No Update required for the K8sFSC NodeTopology API")
				}

				// check node topology
				if c.nodeTopo == nil {
					log.Info("NodeTopology does not exist")
				} else {
					log.Infof("NodeTopology exists: %v", *c.nodeTopo)
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
				log.Info("Wrong message received")
			}
		case <-stopCh:
			log.Info("Stopping timeout controller")
			return
		}
	}

}
