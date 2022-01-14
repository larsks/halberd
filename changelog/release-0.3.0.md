Halberd 0.3.0, the Possibly Useful release

This release introduces:

- The `--namespaced` and `--non-namespaced` flags, allowing you to select those
  subclasses of resources. The namespaced-ness of resources is determined by
  the resource type cache built when you use the `--update` or `--update-only`
  flags.

- The `--add-kustomization` flag, which instructs Halberd to generate a
  `kustomization.yaml` for each manifest it writes out.
