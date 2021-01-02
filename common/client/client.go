package client

import (
	"fmt"
	"time"

	fscclient "github.com/henderiw/fsc-lib-go/pkg/client/clientset/versioned"
	netclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

// Info struct
type Info struct {
	K8sClient *kubernetes.Clientset
	NetClient *netclient.Clientset
	FscClient *fscclient.Clientset
}

func getClientConfig(kubeconfig *string) (*rest.Config, error) {
	if *kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}
	return rest.InClusterConfig()
}

// GetClients gets client info from kubeconfig
func GetClients(kubeconfig *string) (*Info, error) {
	klog.Infof("GetClients: %s", *kubeconfig)

	// Build the client config - optionally using a provided kubeconfig file.
	config, err := getClientConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Cannot get k8s config %s", err)
	}

	config.Timeout = time.Minute

	// creates the clientset
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Cannot get k8s client %s", err)
	}

	netclient, err := netclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Cannot get cncf client %s", err)
	}

	fscclient, err := fscclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Cannot get fsc client %s", err)
	}

	return &Info{
		K8sClient: k8sclient,
		NetClient: netclient,
		FscClient: fscclient,
	}, nil
}
