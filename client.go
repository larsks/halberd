package main

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
)

func getClient() (*discovery.DiscoveryClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, err
}
