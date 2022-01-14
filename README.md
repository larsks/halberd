# Halberd: A tool for splitting Helms

Halberd splits a YAML document containing multiple Kubernetes resources
into individual files, organized following Operate First standards.

## Synopsis

```
Usage:
  halberd [flags]

Flags:
  -k, --add-kustomize          Create kustomization.yaml files
  -r, --api-resources string   api resources information (default "/home/lars/.config/halberd/resources.yaml")
  -d, --directory string       target directory (default ".")
  -h, --help                   help for halberd
      --kubeconfig string      absolute path to the kubeconfig file (default "/home/lars/.kube/config")
  -n, --namespaced             Only emit namespaced resources
  -N, --non-namespaced         Only emit non-namespaced resources
      --update                 Update resource cache
      --update-only            Update resource cache and exit
  -v, --verbose count          Increase log verbosity
      --version                Display version information
```

## Examples

To organize a collection of manifests on stdin:

```
something-that-emits-manifests | halberd
```

To organize a collection of manifests in multiple files:

```
halberd file1.yaml file2.yaml
```

## Resource cache

Halberd is built with an embedded list of resource types that it uses
when organizing manifests. This list may not match the list of
resources available on your cluster. Halberd is able to query your
cluster for a list of available resources and cache them locally.

You can either add the `--update` flag to any other operation, for
example:

```
halberd --update manifests.yaml
```

Or you can use the `--update-only` flag to have Halberd update the
resource cache and exit:

```
halberd --update-only
```
