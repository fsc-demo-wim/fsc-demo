package status

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/klog"

	"k8s.io/client-go/kubernetes"
)

// RunHealthChecks runs the controller health checks.
func RunHealthChecks(ctx context.Context, s *Status, k8sClientset *kubernetes.Clientset) {
	s.SetReady("KubeAPIServer", false, "initialized to false")

	// Loop forever and perform healthchecks.
	for {
		// Kube-apiserver HealthCheck
		healthStatus := 0
		k8sCheckDone := make(chan interface{}, 1)
		go func(k8sCheckDone <-chan interface{}) {
			time.Sleep(2 * time.Second)
			select {
			case <-k8sCheckDone:
				// The check has completed.
			default:
				// Check is still running, so report not ready.
				s.SetReady(
					"KubeAPIServer",
					false,
					fmt.Sprintf("Error reaching apiserver: taking a long time to check apiserver"),
				)
			}
		}(k8sCheckDone)
		k8sClientset.Discovery().RESTClient().Get().AbsPath("/healthz").Do(ctx).StatusCode(&healthStatus)
		k8sCheckDone <- nil
		if healthStatus != http.StatusOK {
			klog.Errorf("Failed to reach apiserver")
			s.SetReady(
				"KubeAPIServer",
				false,
				fmt.Sprintf("Error reaching apiserver: %v with http status code: %d", nil, healthStatus),
			)
		} else {
			s.SetReady("KubeAPIServer", true, "")
		}

		time.Sleep(10 * time.Second)
	}
}
