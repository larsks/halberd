package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"os"

	"github.com/larsks/halberd/version"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

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
	gitAddFlag                 bool
	namespacedOnlyFlag         bool
	nonNamespacedOnlyFlag      bool
	versionFlag                bool
)

func GitAddFile(path string) error {
	log.Debug().Str("path", path).Msgf("adding file to git repository")
	cmd := exec.Command("git", "add", path)
	err := cmd.Run()
	return err
}

func Split(reader io.Reader) int {
	dec := yaml.NewDecoder(reader)
	count := 0

	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal().Msgf("failed to decode yaml: %v", err)
		}

		res, err := ResourceFromNode(&node)
		if err != nil {
			var e *InvalidResourceError
			if errors.As(err, &e) {
				log.Warn().Err(err).Msgf("failed to decode resource")
				continue
			} else {
				log.Fatal().Err(err).Msgf("failed to decode resource")
			}
		}

		if namespacedOnlyFlag && !res.Info.Namespaced {
			continue
		}

		if nonNamespacedOnlyFlag && res.Info.Namespaced {
			continue
		}

		path := res.Path()
		log.Debug().Msgf("putting %s/%s in %s", res.Definition.Kind, res.Definition.Metadata.Name, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			log.Fatal().Err(err).Msgf("failed to create directory %s", path)
		}

		var content bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&content)
		yamlEncoder.SetIndent(2)
		if err := yamlEncoder.Encode(&node); err != nil {
			log.Fatal().Err(err).Msgf("failed to marshal yaml")
		}

		err = os.WriteFile(path, content.Bytes(), 0644)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to write file")
		}

		if gitAddFlag {
			if err = GitAddFile(path); err != nil {
				log.Fatal().Err(err).Msgf("failed to add file to git repository")
			}
		}

		if generateKustomizeFlag {
			k := NewKustomization()
			k.AddResource(filepath.Base(path))
			kPath := filepath.Join(filepath.Dir(path), "kustomization.yaml")
			if err := k.Write(kPath); err != nil {
				log.Fatal().Err(err).Msgf("failed to write kustomization")
			}

			if gitAddFlag {
				if err = GitAddFile(kPath); err != nil {
					log.Fatal().Err(err).Msgf("failed to add kustomization to git repository")
				}
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

			if versionFlag {
				version.ShowVersion()
				return nil
			}

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
						log.Error().Err(err)
						return err
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
	rootCmd.Flags().BoolVarP(&gitAddFlag, "git-add", "g", false, "Add generated files to git repository")
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
	rootCmd.Flags().BoolVarP(&namespacedOnlyFlag, "namespaced", "n", false, "Only emit namespaced resources")
	rootCmd.Flags().BoolVarP(&nonNamespacedOnlyFlag, "non-namespaced", "N", false, "Only emit non-namespaced resources")
	rootCmd.Flags().BoolVar(&versionFlag, "version", false, "Display version information")

	return &rootCmd
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	cobra.CheckErr(NewCmdRoot().Execute())
}
