package multus

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fsc-demo-wim/fsc-demo/common/controller"
	"github.com/fsc-demo-wim/fsc-demo/common/msg"
	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	netclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	uruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Config struct
type Config struct {
	CniVersion string `json:"cniVersion,omitempty"`
	LogFile    string `json:"LogFile,omitempty"`
	LogLevel   string `json:"LogLevel,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Master     string `json:"master,omitempty"`
	Vlan       int    `json:"vlan"`
	L2Enable   bool   `json:"l2enable"`
	Ipam       struct {
		Type       string `json:"type,omitempty"`
		Subnet     string `json:"subnet,omitempty"`
		RangeStart string `json:"rangeStart,omitempty"`
		RangeEnd   string `json:"rangeEnd,omitempty"`
	} `json:"ipam"`
}

// multus struct
type multus map[string]NetwAtt

// NetwAtt struct
type NetwAtt struct {
	Kind         string
	Vlan         int
	ItfceName    string
	ResourceName string
	Ns           string
}

// SriovCfg variable
var SriovCfg map[string]string

const workerthreads = 1

// parseConfig function
func parseConfig(obj interface{}) (*NetwAtt, error) {
	// Parse information from the Network Attachement object
	specConfig := obj.(*v1.NetworkAttachmentDefinition).Spec.Config
	annotations := obj.(*v1.NetworkAttachmentDefinition).GetAnnotations()
	ns := obj.(*v1.NetworkAttachmentDefinition).GetNamespace()
	var c Config
	var itfceName string
	err := json.Unmarshal([]byte(specConfig), &c)
	if err != nil {
		return nil, err
	}
	log.Debugf("Multus NetwAtachDef info: %s %s %s %d", c.Name, c.Type, c.Master, c.Vlan)

	// Initialize new NetwAtt struct with the information received from the Multus API server
	na := new(NetwAtt)
	switch c.Type {
	case "sriov":
		var rn string
		var ok bool
		if rn, ok = annotations["k8s.v1.cni.cncf.io/resourceName"]; ok {
			log.Debugf("ResourceName: %s", rn)
			itfceName = SriovCfg[rn]

		}
		log.Debugf("NameSpace %s Interfaces to be monitored: %s, vlan: %d", ns, itfceName, c.Vlan)
		log.Debugf("ResourceName2: %s", rn)
		na = &NetwAtt{
			Kind:         c.Type,
			Vlan:         c.Vlan,
			ResourceName: rn,
			Ns:           ns,
		}
	case "ipvlan", "macvlan":
		split := strings.Split(c.Master, ".")
		if len(split) > 1 {
			vlan, err := strconv.Atoi(split[1])
			if err != nil {
				log.Error(err)
			}
			log.Debugf("NameSpace %s Interfaces to be monitored: %s, vlan: %d", ns, split[0], vlan)
			na = &NetwAtt{
				Kind:      c.Type,
				Vlan:      vlan,
				ItfceName: split[0],
				Ns:        ns,
			}
		} else {
			log.Debugf("NameSpace %s Interfaces to be monitored: %s, vlan: %d", ns, split[0], 0)
			na = &NetwAtt{
				Kind:      c.Type,
				Vlan:      0,
				ItfceName: split[0],
				Ns:        ns,
			}
		}
	default:
	}
	return na, nil
}

// multusController implements the Controller interface for managing Kubernetes object
// and syncing them to the datastore as Profiles.
type multusController struct {
	indexer  cache.Indexer
	informer cache.Controller
	ctx      context.Context
	queue    workqueue.RateLimitingInterface
	workCh   chan msg.CMsg
}

// NewController returns a controller which manages multus objects.
// ..
func NewController(ctx context.Context, Client *netclient.Clientset) controller.Controller {
	//SriovCfg = sriovCfg
	// Create a multus watcher.
	listWatcher := cache.NewListWatchFromClient(Client.K8sCniCncfIoV1().RESTClient(), "network-attachment-definitions", "", fields.Everything())

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the cache to kubernetes cache with the help of an informer. This way we make sure that
	// whenever the kubernetes cache is updated, changes get reflected in the cache as well.
	indexer, informer := cache.NewIndexerInformer(listWatcher, &v1.NetworkAttachmentDefinition{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				log.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			log.Infof("Got ADD event for network-attachment-definitions: %s", key)

		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err != nil {
				log.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			log.Infof("Got UPDATE event for network-attachment-definitions: %s", key)

		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				log.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			log.Infof("Got DELETE event for network-attachment-definitions: %s", key)

		},
	}, cache.Indexers{})

	return &multusController{indexer, informer, ctx, queue, nil}
}

// Run starts the controller.
func (c *multusController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {
	c.workCh = workCh
	defer uruntime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	log.Info("Starting Multus controller")

	// Wait till k8s cache is synced
	go c.informer.Run(stopCh)
	log.Info("Waiting to sync with Kubernetes API (Multus)")

	for !c.informer.HasSynced() {
	}
	log.Infof("Finished syncing with Kubernetes API (Multus)")

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	// Start a number of worker threads to read from the queue.
	for i := 0; i < workerthreads; i++ {
		go c.runWorker()
	}
	log.Info("Multus controller is now running")

	<-stopCh
	log.Info("Stopping Multus controller")
}

func (c *multusController) runWorker() {
	for c.processNextItem() {
	}
}

// processNextItem waits for an event on the output queue from the resource cache and syncs
// any received keys to the datastore.
func (c *multusController) processNextItem() bool {
	// Wait until there is a new item in the work queue.
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	// Indicate that we're done processing this key, allowing for safe parallel processing such that
	// two objects with the same key are never processed in parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.process(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)

	return true
}

// process is the business logic of the controller. We parse the multusconfig and send a message to the
// fscagent controller where the real processing happens
func (c *multusController) process(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		log.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		// Below we will warm up our cache with a Pod, so that we will see a delete for one pod
		log.Infof("network-attachment-definition %s does not exist anymore", key)
		nwa := make(map[string]NetwAtt)
		nwa[key] = NetwAtt{}
		m := msg.CMsg{
			Type:     msg.MultusDelete,
			KeyValue: nwa,
			RespChan: nil,
		}
		c.workCh <- m
	} else {
		na, err := parseConfig(obj)
		if err != nil {
			log.Error(err)
		} else {
			nwa := make(map[string]NetwAtt)
			fmt.Printf("NetwAttach send to worker: %v\n", *na)
			nwa[key] = *na
			m := msg.CMsg{
				Type:     msg.MultusUpdate,
				KeyValue: nwa,
				RespChan: nil,
			}
			c.workCh <- m
		}

	}
	return nil
}

// handleErr handles errors which occur while processing a key received from the resource cache.
// For a given error, we will re-queue the key in order to retry the datastore sync up to 5 times,
// at which point the update is dropped.
func (c *multusController) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		log.Errorf("Error syncing Profile %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)

	// Report to an external entity that, even after several retries, we could not successfully process this key
	uruntime.HandleError(err)
	log.Errorf("Dropping %q out of the queue: %v", key, err)
}
