package k8sclient

import (
	"flag"
	"fmt"
	"os"

	"path/filepath"
	"k8s.io/client-go/util/homedir"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeConfig() (*restclient.Config, error) {
	var kubeconfigPath *string

	defaultKubeconfigPath := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfigPath = filepath.Join(home, ".kube", "config")
	}
	kubeconfigPath = flag.String("kubeconfig", defaultKubeconfigPath, "(optional) absolute path to the kubeconfig file")
	flag.Parse()

	// test if kubeconfig actually exists
	if _, err := os.Stat(*kubeconfigPath); err != nil {
		if os.IsNotExist(err) {
			// it does not exist. Attempt to load in-cluster credentials
			kubeconfig, err := restclient.InClusterConfig()
			if err != nil {
				return nil, fmt.Errorf("Error creating in-cluster config: %s", err.Error())
			}
			return kubeconfig, nil
		}
	}
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("Error creating config from kubeconfig file: %s", err.Error())
	}
	return kubeconfig, nil
}
