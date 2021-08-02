package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed data/resources.yaml
var apiResourcesData []byte
var apiResources []APIResource
var apiResourceMap map[string]APIResource = make(map[string]APIResource, 0)

type (
	kvmap map[string]string

	// Metadata represents the metadata section of a Kubernetes resource.
	Metadata struct {
		Name        string
		Annotations kvmap `yaml:",omitempty"`
		Labels      kvmap `yaml:",omitempty"`
	}

	// Resource represents the core attributes of a Kubernetes resource. This
	// struct is embedded in all other Kubernetes resource models.
	Resource struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string
		Metadata   Metadata `yaml:",omitempty"`
	}

	APIResource struct {
		Name       string
		Namespaced bool
		Kind       string
		APIVersion string `yaml:"apiVersion"`
		APIGroup   string `yaml:"apiGroup"`
	}
)

func (r APIResource) Key() string {
	return fmt.Sprintf("%s/%s", r.APIGroup, r.Kind)
}

func (r Resource) Group() (group string) {
	parts := strings.Split(r.APIVersion, "/")
	if len(parts) > 1 {
		group = parts[0]
	} else {
		group = "core"
	}

	return group
}

func (r Resource) Key() string {
	return fmt.Sprintf("%s/%s", r.Group(), r.Kind)
}

func (r Resource) Path() string {
	spec, exists := apiResourceMap[r.Key()]
	if !exists {
		log.Fatalf("%s: unknown resource", r.Key())
	}
	return fmt.Sprintf(
		"%s/%s/%s/%s.yaml",
		spec.APIGroup,
		strings.ToLower(spec.Name),
		r.Metadata.Name,
		strings.ToLower(spec.Kind),
	)
}

func readApiResources() {
	err := yaml.Unmarshal(apiResourcesData, &apiResources)
	if err != nil {
		panic(err)
	}
	log.Printf("read %d api resources", len(apiResources))

	for _, r := range apiResources {
		apiResourceMap[r.Key()] = r
	}
}

func Split() {
	readApiResources()

	reader := io.Reader(os.Stdin)
	dec := yaml.NewDecoder(reader)

	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}

		var res Resource
		err = node.Decode(&res)
		if err != nil {
			panic(err)
		}

		path := res.Path()
		log.Printf("putting %s/%s in %s", res.Kind, res.Metadata.Name, path)
		os.MkdirAll(filepath.Dir(path), 0755)

		content, err := yaml.Marshal(&node)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(path, content, 0644)
		if err != nil {
			panic(err)
		}
	}
}

var apiResourcePath string
var targetDir string

var rootCmd = &cobra.Command{
	Use:   "halberd",
	Args:  cobra.MaximumNArgs(1),
	Short: "A tool for breaking Helms",
	Long: `A tool for breaking Helms

Halberd splits a YAML document containing multiple Kubernetes resources
into individual files, organized following Operate First standards.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := os.Chdir(targetDir)
		if err != nil {
			return fmt.Errorf("unable to access %s: %w", targetDir, err)
		}

		Split()

		return nil
	},
}

func main() {
	rootCmd.Flags().StringVarP(
		&apiResourcePath, "api-resources", "r", "", "api resources information")
	rootCmd.Flags().StringVarP(
		&targetDir, "directory", "d", ".", "target directory")
	cobra.CheckErr(rootCmd.Execute())
}
