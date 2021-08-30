package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"os"

	"github.com/larsks/halberd/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"k8s.io/client-go/util/homedir"
)

var (
	updateResourcesFlag        bool
	updateResourcesAndExitFlag bool
	apiResourcesPath           string
	kubeconfig                 string
	targetDir                  string
	verbosity                  int
	generateKustomizeFlag      bool
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
		log.Fatal().Msgf("%s: unknown resource", r.Key())
	}
	return fmt.Sprintf(
		"%s/%s/%s/%s.yaml",
		spec.APIGroup,
		strings.ToLower(spec.Name),
		r.Metadata.Name,
		strings.ToLower(spec.Kind),
	)
}

func Split(reader io.Reader) int {
	dec := yaml.NewDecoder(reader)
	count := 0

	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatal().Msgf("failed to decode yaml: %v", err)
		}

		var res Resource
		err = node.Decode(&res)
		if err != nil {
			log.Fatal().Msgf("failed to decode resource: %v", err)
		}
		if res.APIVersion == "" {
			log.Warn().Msgf("skipping invalid resource")
			continue
		}

		path := res.Path()
		log.Debug().Msgf("putting %s/%s in %s", res.Kind, res.Metadata.Name, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			log.Fatal().Err(err).Msgf("failed to create directory %s", path)
		}

		content, err := yaml.Marshal(&node)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to marshal yaml")
		}

		err = ioutil.WriteFile(path, content, 0644)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to write file")
		}

		if generateKustomizeFlag {
			k := NewKustomization()
			k.AddResource(filepath.Base(path))
			kPath := filepath.Join(filepath.Dir(path), "kustomization.yaml")
			if err := k.Write(kPath); err != nil {
				log.Fatal().Err(err).Msgf("failed to write kustomization")
			}
		}

		count++
	}

	return count
}

func setLogLevel() {
	var logLevel zerolog.Level
	switch verbosity {
	case 0:
		logLevel = zerolog.WarnLevel
	case 1:
		logLevel = zerolog.InfoLevel
	default:
		logLevel = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(logLevel)
}

func NewCmdRoot() *cobra.Command {
	rootCmd := cobra.Command{
		Use:           "halberd",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "A tool for breaking Helms",
		Long: `A tool for breaking Helms

Halberd splits a YAML document containing multiple Kubernetes resources
into individual files, organized following Operate First standards.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var readers []io.Reader

			setLogLevel()
			log.Info().Msgf("Halberd build %s", version.BuildRef)

			if updateResourcesAndExitFlag {
				updateResourcesFlag = true
			}

			if updateResourcesFlag {
				log.Info().Msgf("updating api resource cache")
				err := updateResources()

				if updateResourcesAndExitFlag {
					return err
				}

				if err != nil {
					log.Warn().Err(err).Msgf("failed to update resource cache")
				}
			}
			if err := readApiResources(); err != nil {
				return err
			}

			if len(args) > 0 {
				for _, path := range args {
					log.Info().Msgf("reading manifests from %s", path)
					f, err := os.Open(path)
					if err != nil {
						log.Fatal().Err(err)
					}

					defer f.Close()
					readers = append(readers, io.Reader(f))
				}
			} else {
				log.Info().Msgf("reading manifests from stdin")
				readers = append(readers, io.Reader(os.Stdin))
			}

			err := os.Chdir(targetDir)
			if err != nil {
				return fmt.Errorf("unable to access %s: %w", targetDir, err)
			}

			total := 0
			for _, reader := range readers {
				total += Split(reader)
			}

			log.Info().Msgf("processed %d resources", total)

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

	rootCmd.Flags().BoolVarP(&generateKustomizeFlag, "add-kustomize", "k", false, "Create kustomization.yaml files")
	rootCmd.Flags().StringVarP(
		&apiResourcesPath, "api-resources", "r", defaultResources, "api resources information")
	rootCmd.Flags().StringVarP(
		&targetDir, "directory", "d", ".", "target directory")
	rootCmd.Flags().BoolVar(
		&updateResourcesFlag, "update", false, "Update resource cache")
	rootCmd.Flags().BoolVar(
		&updateResourcesAndExitFlag, "update-only", false, "Update resource cache and exit")
	rootCmd.Flags().CountVarP(
		&verbosity, "verbose", "v", "Increase log verbosity")

	return &rootCmd
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	cobra.CheckErr(NewCmdRoot().Execute())
}
