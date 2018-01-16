// Package k8s provides a PodLister and PodDeleter that talks to
// a real Kubernetes
package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is a wrapper around a Kubernetes cluster
type Client struct {
	client *kubernetes.Clientset
}

// New creates and returns a new client. If kubeconfig is not define, then
// an in-cluster client is created. context is only used if kubeconfig
// is specified and sets the k8s context - if blank, current context from the
// config file is used.
func New(kubeconfig string, context string) (*Client, error) {
	if kubeconfig == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create an in-cluster config")
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create an in-cluster client")
		}
		return &Client{clientset}, nil
	}
	config, err := k8sConfig(kubeconfig, context)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create a config from %q", kubeconfig)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create a client from %q", kubeconfig)
	}
	return &Client{clientset}, nil

}

func k8sConfig(kubeconfig string, context string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: context},
	).ClientConfig()
}

// ListPods will return a list of Pods in a namespace, optionally using a label selector.
// Empty namespace means all namespaces
func (c *Client) ListPods(namespace string, selector string) ([]v1.Pod, error) {
	pods, err := c.client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	return pods.Items, nil
}

// DeletePod attempts to delete a single pod
func (c *Client) DeletePod(namespace string, name string) error {
	// XXX: Do we need any delete options?
	// https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#DeleteOptions
	// we do not wrap the error here, as the caller may need to check it directly
	return c.client.CoreV1().Pods(namespace).Delete(name, nil)
}
