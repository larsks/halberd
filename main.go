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

	"k8s.io/client-go/util/homedir"
)

var (
	//go:embed data/resources.yaml
	apiResourcesData       []byte
	apiResources           []APIResource
	apiResourcesMap        map[string]APIResource = make(map[string]APIResource)
	updateResources        bool
	updateResourcesAndExit bool
	apiResourcesPath       string
	kubeconfig             string
	targetDir              string
)

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
		log.Printf("reading api resources from %s", apiResourcesPath)
		data, err := ioutil.ReadFile(apiResourcesPath)
		if err != nil {
			log.Printf("unable to open resource cache %s; using embedded data",
				apiResourcesPath)
		} else {
			apiResourcesData = data
		}
	} else {
		log.Printf("reading resources from embedded data")
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

func NewCmdRoot() *cobra.Command {
	rootCmd := cobra.Command{
		Use:   "halberd",
		Args:  cobra.ArbitraryArgs,
		Short: "A tool for breaking Helms",
		Long: `A tool for breaking Helms

Halberd splits a YAML document containing multiple Kubernetes resources
into individual files, organized following Operate First standards.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var readers []io.Reader

			if updateResourcesAndExit {
				updateResources = true
			}

			if updateResources {
				log.Printf("updating api resource cache")
				if err := UpdateResources(); err != nil {
					panic(err)
				}

				if updateResourcesAndExit {
					return nil
				}
			}
			readApiResources()

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

	var defaultKubeconfig string
	var defaultResources string

	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
		defaultResources = filepath.Join(home, ".config", "halberd", "resources.yaml")
	}

	rootCmd.Flags().StringVar(
		&kubeconfig, "kubeconfig", defaultKubeconfig, "absolute path to the kubeconfig file")

	rootCmd.Flags().StringVarP(
		&apiResourcesPath, "api-resources", "r", defaultResources, "api resources information")
	rootCmd.Flags().StringVarP(
		&targetDir, "directory", "d", ".", "target directory")
	rootCmd.Flags().BoolVar(
		&updateResources, "update", false, "Update resource cache")
	rootCmd.Flags().BoolVar(
		&updateResourcesAndExit, "update-only", false, "Update resource cache and exit")

	return &rootCmd
}

func main() {
	log.Printf("Halberd build %s", version.BuildRef)
	cobra.CheckErr(NewCmdRoot().Execute())
}
