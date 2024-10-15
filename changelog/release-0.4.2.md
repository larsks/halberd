Halberd 0.4.2, really minor cleanups

- Fix linting errors

  While this code builds, golangci-lint running in github complains about
  references to `yaml` when we're importing `gopkg.in/yaml.v3`. Fix the
  imports to explicitly use the alias `yaml`.

- Replace git url with https url for pre-commit repository
