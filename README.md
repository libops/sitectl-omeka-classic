# sitectl-omeka-classic

`sitectl-omeka-classic` is the LibOps sitectl plugin for Omeka Classic.

It registers a first-class create definition for `https://github.com/libops/omeka-classic` so the stack can be installed with:

```bash
sitectl create omeka-classic
```

It also provides context-aware helpers:

- `sitectl omeka-classic build`
- `sitectl omeka-classic init`
- `sitectl omeka-classic up`
- `sitectl omeka-classic down`
- `sitectl omeka-classic status`
- `sitectl omeka-classic logs [SERVICE...]`
- `sitectl omeka-classic rollout`

Omeka Classic-specific helpers:

- `sitectl omeka-classic api get RESOURCE [ID]`
- `sitectl omeka-classic api request METHOD PATH`
- `sitectl omeka-classic items [ID]`
- `sitectl omeka-classic collections [ID]`
- `sitectl omeka-classic files [ID]`
- `sitectl omeka-classic tags [ID]`
- `sitectl omeka-classic users [ID]`
- `sitectl omeka-classic element-sets [ID]`
- `sitectl omeka-classic elements [ID]`
- `sitectl omeka-classic item-types [ID]`
- `sitectl omeka-classic site`

API helpers accept `--url`, `--key`, and repeated `--query name=value` flags.
