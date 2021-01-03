package configmap

import (
	"context"
	"fmt"

	"github.com/fsc-demo-wim/fsc-demo/common/controller"
	"github.com/fsc-demo-wim/fsc-demo/common/msg"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	uruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const workerthreads = 1

// configMapController implements the Controller interface for managing Kubernetes configmap
// information related to sriov
type configMapController struct {
	indexer     cache.Indexer
	informer    cache.Controller
	ctx         context.Context
	queue       workqueue.RateLimitingInterface
	sriovConfig *map[string]*string
	workCh      chan msg.CMsg
}

// NewController returns a controller which manages configmaps objects.
// ..
func NewController(ctx context.Context, k8sClient *kubernetes.Clientset) controller.Controller {
	sc := make(map[string]*string)
	// Create a configmap watcher.
	listWatcher := cache.NewListWatchFromClient(k8sClient.CoreV1().RESTClient(), "configmaps", "kube-system", fields.OneTermEqualSelector("metadata.name", "sriovdp-config"))

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the cache to kubernetes cache with the help of an informer. This way we make sure that
	// whenever the kubernetes cache is updated, changes get reflected in the cache as well.
	indexer, informer := cache.NewIndexerInformer(listWatcher, &v1.ConfigMap{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
			log.Debugf("Got ADD event for configmap: %s", key)
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
			log.Debugf("Got UPDATE event for configmap object: %s", key)
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
			log.Debugf("Got DELETE event for configmap object: %s", key)

		},
	}, cache.Indexers{})

	return &configMapController{indexer, informer, ctx, queue, &sc, nil}
}

// Run starts the controller.
func (c *configMapController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {
	c.workCh = workCh
	defer uruntime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	log.Info("Starting configmap controller")

	go c.informer.Run(stopCh)

	// Wait for all caches to be synced, before processing items from the queue is started
	for !c.informer.HasSynced() {
	}
	log.Info("Finished syncing with Kubernetes API (configmap)")

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	// Start a number of worker threads to read from the queue.
	for i := 0; i < workerthreads; i++ {
		go c.runWorker()
	}
	log.Info("Configmap controller is now running")

	<-stopCh
	log.Info("Stopping Configmap controller")
}

func (c *configMapController) runWorker() {
	for c.processNextItem() {
	}
}

// processNextItem waits for an event on the output queue from the resource cache and syncs
// any received keys to the datastore.
func (c *configMapController) processNextItem() bool {
	// Wait until there is a new item in the work queue.
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.process(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)

	return true
}

// process is the business logic of the controller.We parse the sriov configmap and send a message to the
// fscagent controller where the real processing happens
func (c *configMapController) process(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		log.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		log.Infof("ConfigMap %s does not exist anymore", key)
		m := msg.CMsg{
			Type:     msg.SriovDelete,
			KeyValue: *c.sriovConfig,
			RespChan: nil,
		}
		c.workCh <- m
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a Pod was recreated with the same name
		log.Infof("Sync/Add/Update for configmap %s", obj.(*v1.ConfigMap).GetName())

		if err = sriovParseConfig(obj.(*v1.ConfigMap).Data["config.json"], c.sriovConfig); err != nil {
			log.Error(err)
		} else {
			fmt.Printf("sriovConfig: %v \n", *c.sriovConfig)
			m := msg.CMsg{
				Type:     msg.SriovUpdate,
				KeyValue: *c.sriovConfig,
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
func (c *configMapController) handleErr(err error, key interface{}) {
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
		log.Errorf("Error syncing %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)

	// Report to an external entity that, even after several retries, we could not successfully process this key
	uruntime.HandleError(err)
	log.Errorf("Dropping Profile %q out of the queue: %v", key, err)
}
