Halberd 0.4.1, the Your-place-or-whitespace release

The default indentation used by the `yaml.v3` package would result in errors
from a default [`yamllint`][yamllint] configuration. With this release, we explicitly
set an indent of two spaces in order to avoid the effort of filtering the
generated output through something like `yq -y`.

[yamllint]: https://github.com/adrienverge/yamllint
