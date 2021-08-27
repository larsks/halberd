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

	"github.com/larsks/halberd/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed data/resources.yaml
var apiResourcesData []byte
var apiResources []APIResource
var apiResourcesMap map[string]APIResource = make(map[string]APIResource)

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
	return fmt.Sprintf("%s/%s/%s", r.APIGroup, r.APIVersion, r.Kind)
}

func (r Resource) Group() (group string) {
	parts := strings.Split(r.APIVersion, "/")
	if len(parts) > 1 {
		return r.APIVersion
	} else {
		return fmt.Sprintf("core/%s", r.APIVersion)
	}
}

func (r Resource) Key() string {
	return fmt.Sprintf("%s/%s", r.Group(), r.Kind)
}

func (r Resource) Path() string {
	spec, exists := apiResourcesMap[r.Key()]
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
	if apiResourcesPath != "" {
		data, err := ioutil.ReadFile(apiResourcesPath)
		if err != nil {
			log.Fatalf("failed to open %s: %v",
				apiResourcesPath, err)
		}

		apiResourcesData = data
	}

	err := yaml.Unmarshal(apiResourcesData, &apiResources)
	if err != nil {
		log.Fatalf("failed to read api resources: %v", err)
	}
	log.Printf("read %d api resources", len(apiResources))

	for _, r := range apiResources {
		apiResourcesMap[r.Key()] = r
	}
}

func Split(reader io.Reader) {
	dec := yaml.NewDecoder(reader)

	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("failed to decode yaml: %v", err)
		}

		var res Resource
		err = node.Decode(&res)
		if err != nil {
			log.Fatalf("failed to decode resource: %v", err)
		}
		if res.APIVersion == "" {
			log.Printf("skipping invalid resource")
			continue
		}

		path := res.Path()
		log.Printf("putting %s/%s in %s", res.Kind, res.Metadata.Name, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			log.Fatalf("failed to create directory %s: %v", path, err)
		}

		content, err := yaml.Marshal(&node)
		if err != nil {
			log.Fatalf("failed to marshal yaml: %v", err)
		}

		err = ioutil.WriteFile(path, content, 0644)
		if err != nil {
			log.Fatalf("failed to write file: %v", err)
		}
	}
}

var apiResourcesPath string
var targetDir string

var rootCmd = &cobra.Command{
	Use:   "halberd",
	Args:  cobra.ArbitraryArgs,
	Short: "A tool for breaking Helms",
	Long: `A tool for breaking Helms

Halberd splits a YAML document containing multiple Kubernetes resources
into individual files, organized following Operate First standards.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var readers []io.Reader

		if len(args) > 0 {
			for _, path := range args {
				f, err := os.Open(path)
				if err != nil {
					log.Fatal(err)
				}

				defer f.Close()
				readers = append(readers, io.Reader(f))
			}
		} else {
			log.Println("Reading from stdin")
			readers = append(readers, io.Reader(os.Stdin))
		}

		readApiResources()

		err := os.Chdir(targetDir)
		if err != nil {
			return fmt.Errorf("unable to access %s: %w", targetDir, err)
		}

		for _, reader := range readers {
			Split(reader)
		}

		return nil
	},
}

func main() {
	log.Printf("Halberd build %s", version.BuildRef)

	rootCmd.Flags().StringVarP(
		&apiResourcesPath, "api-resources", "r", "", "api resources information")
	rootCmd.Flags().StringVarP(
		&targetDir, "directory", "d", ".", "target directory")
	cobra.CheckErr(rootCmd.Execute())
}
