package workload

import (
	"context"
	"fmt"

	"github.com/henderiw/fsc-demo/common/controller"
	"github.com/henderiw/fsc-demo/common/msg"
	apiv1 "github.com/henderiw/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	fscclient "github.com/henderiw/fsc-lib-go/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	uruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const workerthreads = 1

// workloadController implements the Controller interface for managing Kubernetes object
// and syncing them to the datastore as Profiles.
type workloadController struct {
	indexer  cache.Indexer
	informer cache.Controller
	ctx      context.Context
	queue    workqueue.RateLimitingInterface
	workCh   chan msg.CMsg
}

// NewController returns a controller which manages multus objects.
// ..
func NewController(ctx context.Context, Client *fscclient.Clientset) controller.Controller {
	// Create a watcher.
	listWatcher := cache.NewListWatchFromClient(Client.FscV1().RESTClient(), "workloads", "", fields.Everything())

	// create the workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the cache to kubernetes cache with the help of an informer. This way we make sure that
	// whenever the kubernetes cache is updated, changes get reflected in the cache as well.
	indexer, informer := cache.NewIndexerInformer(listWatcher, &apiv1.WorkLoad{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				klog.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			klog.Infof("Got ADD event for workloads: %s", key)

		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err != nil {
				klog.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			klog.Infof("Got UPDATE event for workloads: %s", key)

		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				klog.Errorf("Failed to generate key %s", err)
				return
			}
			queue.Add(key)
			klog.Infof("Got DELETE event for workloads: %s", key)

		},
	}, cache.Indexers{})

	return &workloadController{indexer, informer, ctx, queue, nil}
}

// Run starts the controller.
func (c *workloadController) Run(stopCh chan struct{}, workCh chan msg.CMsg) {
	c.workCh = workCh
	defer uruntime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()

	klog.Info("Starting Workload controller...")

	// Wait till k8s cache is synced
	go c.informer.Run(stopCh)
	klog.Info("Waiting to sync with Kubernetes API (Workload)")

	for !c.informer.HasSynced() {
	}
	klog.Infof("Finished syncing with Kubernetes API (Workload)")

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	// Start a number of worker threads to read from the queue.
	for i := 0; i < workerthreads; i++ {
		go c.runWorker()
	}
	klog.Info("Workload controller is now running...")

	<-stopCh
	klog.Info("Stopping Workload controller...")
}

func (c *workloadController) runWorker() {
	for c.processNextItem() {
	}
}

// processNextItem waits for an event on the output queue from the resource cache and syncs
// any received keys to the datastore.
func (c *workloadController) processNextItem() bool {
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
func (c *workloadController) process(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		// Below we will warm up our cache with a Pod, so that we will see a delete for one pod
		klog.Infof("workload %s does not exist anymore", key)
		o := make(map[string]*apiv1.WorkLoad)
		o[key] = &apiv1.WorkLoad{}
		m := msg.CMsg{
			Type:     msg.WorkloadDelete,
			KeyValue: o,
			RespChan: nil,
		}
		c.workCh <- m
	} else {

		o := make(map[string]*apiv1.WorkLoad)
		o[key] = obj.(*apiv1.WorkLoad)
		m := msg.CMsg{
			Type:     msg.WorkloadUpdate,
			KeyValue: o,
			RespChan: nil,
		}
		c.workCh <- m
		klog.Infof("Workload created/updated and send to worker: %v", o)
	}
	return nil
}

// handleErr handles errors which occur while processing a key received from the resource cache.
// For a given error, we will re-queue the key in order to retry the datastore sync up to 5 times,
// at which point the update is dropped.
func (c *workloadController) handleErr(err error, key interface{}) {
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
		klog.Errorf("Error syncing Profile %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)

	// Report to an external entity that, even after several retries, we could not successfully process this key
	uruntime.HandleError(err)
	klog.Errorf("Dropping %q out of the queue: %v", key, err)
}
