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

	APIResourceList []APIResource
)

var (
	//go:embed data/resources.yaml
	apiResourcesData []byte
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

func getResources(client *discovery.DiscoveryClient) (APIResourceList, error) {
	resources, err := client.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	var resourceList APIResourceList

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

func writeResources(resourceList APIResourceList) error {
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

	log.Warn().Msgf("writing resources to %s", apiResourcesPath)
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

func readApiResourcesFromFile(path string) (APIResourceList, error) {
	var apiResources []APIResource

	log.Info().Msgf("reading api resources from %s", apiResourcesPath)

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Warn().Err(err).Msgf("unable to open resource cache %s", path)
		return nil, err
	}

	if err := yaml.Unmarshal(data, &apiResources); err != nil {
		log.Warn().Err(err).Msgf("unable to unmarshal %s", path)
		return nil, err
	}

	return apiResources, nil
}

func readApiResourcesEmbedded() (APIResourceList, error) {
	var apiResources []APIResource

	log.Info().Msgf("reading embedded api resource data")

	err := yaml.Unmarshal(apiResourcesData, &apiResources)
	if err != nil {
		log.Error().Err(err).Msgf("failed to read embedded api resources")
		return nil, err
	}

	return apiResources, nil
}

func readApiResources() error {
	var apiResources APIResourceList

	if apiResourcesPath != "" {
		resources, err := readApiResourcesFromFile(apiResourcesPath)
		if err != nil {
			resources, err = readApiResourcesEmbedded()
			if err != nil {
				return err
			}
		}
		apiResources = resources
	}

	for _, r := range apiResources {
		apiResourcesMap[r.Key()] = r
	}

	return nil
}

func (r APIResource) Key() string {
	return fmt.Sprintf("%s/%s/%s", r.APIGroup, r.APIVersion, r.Kind)
}
