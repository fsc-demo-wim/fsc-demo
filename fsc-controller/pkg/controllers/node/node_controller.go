package node

import (
	"context"
	"fmt"

	"github.com/fsc-demo-wim/fsc-demo/common/controller"
	"github.com/fsc-demo-wim/fsc-demo/common/msg"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	uruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	log "github.com/sirupsen/logrus"
)

const workerthreads = 1

// nodeController implements the Controller interface for managing Kubernetes object
type nodeController struct {
	indexer  cache.Indexer
	informer cache.Controller
	ctx      context.Context
	queue    workqueue.RateLimitingInterface
	workCh   chan msg.CMsg
}

// NewController returns a controller which manages multus objects.
// ..
func NewController(ctx context.Context, Client *kubernetes.Clientset) controller.Controller {
	// Create a watcher.
	listWatcher := cache.NewListWatchFromClient(Client.CoreV1().RESTClient(), "nodes", "", fields.Everything())

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the cache to kubernetes cache with the help of an informer. This way we make sure that
	// whenever the kubernetes cache is updated, changes get reflected in the cache as well.
	indexer, informer := cache.NewIndexerInformer(listWatcher, &v1.Node{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				log.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			log.Infof("Got ADD event for nodes: %s", key)

		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err != nil {
				log.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			log.Infof("Got UPDATE event for nodes: %s", key)

		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				log.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			log.Infof("Got DELETE event for nodes: %s", key)

		},
	}, cache.Indexers{})

	return &nodeController{indexer, informer, ctx, queue, nil}
}

// Run starts the controller.
func (c *nodeController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {
	c.workCh = workCh
	defer uruntime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	log.Info("Starting Node controller")

	// Wait till k8s cache is synced
	go c.informer.Run(stopCh)
	log.Info("Waiting to sync with Kubernetes API (Node)")

	for !c.informer.HasSynced() {
	}
	log.Infof("Finished syncing with Kubernetes API (Node)")

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	// Start a number of worker threads to read from the queue.
	for i := 0; i < workerthreads; i++ {
		go c.runWorker()
	}
	log.Info("Node controller is now running")

	<-stopCh
	log.Info("Stopping Node controller")
}

func (c *nodeController) runWorker() {
	for c.processNextItem() {
	}
}

// processNextItem waits for an event on the output queue from the resource cache and syncs
// any received keys to the datastore.
func (c *nodeController) processNextItem() bool {
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

// process is the business logic of the controller.
func (c *nodeController) process(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		log.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		// Below we will warm up our cache with a Pod, so that we will see a delete for one pod
		log.Infof("node %s does not exist anymore", key)
		o := make(map[string]*v1.Node)
		o[key] = &v1.Node{}
		m := msg.CMsg{
			Type:     msg.NodeDelete,
			KeyValue: o,
			RespChan: nil,
		}
		c.workCh <- m
	} else {

		o := make(map[string]*v1.Node)
		o[key] = obj.(*v1.Node)
		m := msg.CMsg{
			Type:     msg.NodeUpdate,
			KeyValue: o,
			RespChan: nil,
		}
		c.workCh <- m
		log.Infof("Node %s created/updated and send to worker", key)
		//log.Infof("Node %s created/updated and send to worker: %v", key, o)
	}
	return nil
}

// handleErr handles errors which occur while processing a key received from the resource cache.
// For a given error, we will re-queue the key in order to retry the datastore sync up to 5 times,
// at which point the update is dropped.
func (c *nodeController) handleErr(err error, key interface{}) {
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
		log.Errorf("Error syncing  %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)

	// Report to an external entity that, even after several retries, we could not successfully process this key
	uruntime.HandleError(err)
	log.Errorf("Dropping %q out of the queue: %v", key, err)
}
