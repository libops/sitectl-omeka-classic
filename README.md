# sitectl-omeka-classic

`sitectl-omeka-classic` simplifies the creation and operation of repositories created using the [LibOps Omeka Classic template](https://github.com/libops/omeka-classic). It provides sitectl commands for the Omeka Classic API, resource shortcuts, plugin maintenance, validation, and health checks.

Documentation: https://sitectl.libops.io/plugins/omeka-classic

## Requirements

- [`sitectl`](https://sitectl.libops.io/install).
- Docker with the Compose v2 plugin for local Omeka Classic sites.
- No additional app-plugin dependency beyond core `sitectl`.

## Quick Start

Create a local Omeka Classic site from the matching template:

```bash
sitectl create omeka-classic/default \
  --template-repo https://github.com/libops/omeka-classic \
  --path ./my-omeka-classic-site \
  --type local \
  --checkout-source template \
  --default-context
```

The template README is at https://github.com/libops/omeka-classic.

## Basic Operations

Use [`sitectl compose`](https://sitectl.libops.io/commands/compose) to start or inspect the stack:

```bash
sitectl compose up --remove-orphans -d
```

Use [`sitectl healthcheck`](https://sitectl.libops.io/commands/healthcheck) and [`sitectl validate`](https://sitectl.libops.io/commands/validate) to check the site:

```bash
sitectl healthcheck
sitectl validate
```

Use [`sitectl image`](https://sitectl.libops.io/commands/image) for local image or build-arg overrides:

```bash
sitectl image set --tag omeka-classic=nginx-1.30.3-php84
```

Use [`sitectl set`](https://sitectl.libops.io/commands/set) and [`sitectl converge`](https://sitectl.libops.io/commands/converge) for component changes:

```bash
sitectl set upload-limits enabled --max-upload-size 2G --upload-timeout 10m
sitectl converge
```

See the [Omeka Classic plugin docs](https://sitectl.libops.io/plugins/omeka-classic) for lifecycle operations, API helpers, resource shortcuts, and plugin maintenance.

## License

`sitectl-omeka-classic` is licensed under the MIT License.
