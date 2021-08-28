package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"

	"github.com/rs/zerolog/log"

	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	APIResource struct {
		APIGroup   string `yaml:"apiGroup"`
		APIVersion string `yaml:"apiVersion"`
		Kind       string
		Name       string
		Namespaced bool
	}

	ResourceList []APIResource
)

var (
	//go:embed data/resources.yaml
	apiResourcesData []byte
	apiResources     []APIResource
	apiResourcesMap  map[string]APIResource = make(map[string]APIResource)
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

func getResources(client *discovery.DiscoveryClient) (ResourceList, error) {
	resources, err := client.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	var resourceList ResourceList

	for _, rgrp := range resources {
		parts := strings.Split(rgrp.GroupVersion, "/")
		var apigroup, apiversion string
		switch len(parts) {
		case 0:
			return nil, fmt.Errorf("unable to determine groupversion")
		case 1:
			apigroup = "core"
			apiversion = parts[0]
		case 2:
			apigroup = parts[0]
			apiversion = parts[1]
		default:
			return nil, fmt.Errorf("too many components to groupversion")
		}
		for _, rsc := range rgrp.APIResources {
			if rsc.Group == "" {
				rsc.Group = apigroup
			}
			if rsc.Version == "" {
				rsc.Version = apiversion
			}

			entry := APIResource{
				APIGroup:   rsc.Group,
				APIVersion: rsc.Version,
				Kind:       rsc.Kind,
				Name:       rsc.Name,
				Namespaced: rsc.Namespaced,
			}

			resourceList = append(resourceList, entry)
		}
	}

	return resourceList, nil
}

func writeResources(resourceList ResourceList) error {
	resourceJson, err := yaml.Marshal(resourceList)
	if err != nil {
		return err
	}

	apiResourcesDir := filepath.Dir(apiResourcesPath)
	_, err = os.Stat(apiResourcesDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		log.Info().Msgf("creating directory %s", apiResourcesDir)
		err := os.Mkdir(apiResourcesDir, 0o700)
		if err != nil {
			return err
		}
	}

	log.Info().Msgf("writing resources to %s", apiResourcesPath)
	if err := ioutil.WriteFile(apiResourcesPath, resourceJson, 0o644); err != nil {
		return err
	}

	return nil
}

func updateResources() error {
	client, err := getClient()
	if err != nil {
		return err
	}

	resourceList, err := getResources(client)
	if err != nil {
		return err
	}

	if err := writeResources(resourceList); err != nil {
		return err
	}

	return nil
}

func readApiResources() {
	if apiResourcesPath != "" {
		log.Printf("reading api resources from %s", apiResourcesPath)
		data, err := ioutil.ReadFile(apiResourcesPath)
		if err != nil {
			log.Warn().Msgf("unable to open resource cache %s; using embedded data",
				apiResourcesPath)
		} else {
			apiResourcesData = data
		}
	} else {
		log.Printf("reading resources from embedded data")
	}

	err := yaml.Unmarshal(apiResourcesData, &apiResources)
	if err != nil {
		log.Fatal().Msgf("failed to read api resources: %v", err)
	}
	log.Printf("read %d api resources", len(apiResources))

	for _, r := range apiResources {
		apiResourcesMap[r.Key()] = r
	}
}

func (r APIResource) Key() string {
	return fmt.Sprintf("%s/%s/%s", r.APIGroup, r.APIVersion, r.Kind)
}
