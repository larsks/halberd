package main

import (
	"io/ioutil"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type (
	Kustomization struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string
		Resources  []string
	}
)

func NewKustomization() *Kustomization {
	k := Kustomization{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
	}

	return &k
}

func (kustomization *Kustomization) ToYAML() ([]byte, error) {
	content, err := yaml.Marshal(&kustomization)
	return content, err
}

func (kustomization *Kustomization) AddResource(resource string) {
	kustomization.Resources = append(kustomization.Resources, resource)
}

func (kustomization *Kustomization) Write(path string) error {
	log.Debug().Msgf("writing kustomization to %s", path)
	content, err := kustomization.ToYAML()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, content, 0644)
}
