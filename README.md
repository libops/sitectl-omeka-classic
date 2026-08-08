# sitectl-omeka-classic

`sitectl-omeka-classic` simplifies the creation and operation of repositories created using the [LibOps Omeka Classic template](https://github.com/libops/omeka-classic). It provides sitectl commands for the Omeka Classic API, resource shortcuts, plugin maintenance, validation, and health checks.

Documentation: https://sitectl.libops.io/plugins/omeka-classic

## Requirements

- [`sitectl`](https://sitectl.libops.io/install) v1.7.0 or newer provides the RPC verifier SDK; promotion must pin the first core release that also includes `verify --strict` semantics.
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
sitectl verify --strict
```

## Behavioral verification

`sitectl verify --strict` checks the running Omeka Classic version, scoped MariaDB identity, current browser-migration state, default-install application metadata, and files-volume access. It does not depend on the optional REST API, which a default installation disables. The database probe reads the connection selected by the rendered `db.ini`; its password stays inside the container and is never copied into Docker process arguments or verifier output. Production verification is read-only.

Disposable CI may add a reversible service-account file write/read/delete probe:

```bash
sitectl verify --strict --disposable
```

Never use `--disposable` for a retained customer site. Rollout branch tests execute the app-only start, migration-required stop, post-operator retry, and final full-stack start through the Docker command boundary. Hosted acceptance must still exercise a real prior-version database/files fixture, browser migration, public DNS/TLS, mail delivery, admin login, API mutation, and file retrieval.

Use [`sitectl image`](https://sitectl.libops.io/commands/image) for local image or build-arg overrides:

```bash
sitectl image set --tag omeka-classic=3.2.1-php84
```

The plugin intentionally does not register broad development bind mounts: mounting all plugins or themes would hide extensions bundled in the versioned base image. Add custom extensions through the downstream build or an explicit per-extension override.

Use [`sitectl set`](https://sitectl.libops.io/commands/set) for component changes; it updates component-owned files immediately:

```bash
sitectl set ingress enabled --mode https-custom --domain omeka-classic.localhost
sitectl set ingress enabled --trusted-ip 203.0.113.10/32 --max-upload-size 2G --upload-timeout 10m
```

See the [Omeka Classic plugin docs](https://sitectl.libops.io/plugins/omeka-classic) for lifecycle operations, API helpers, resource shortcuts, and plugin maintenance.

## License

`sitectl-omeka-classic` is licensed under the MIT License.
