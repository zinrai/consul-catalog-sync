# consul-catalog-sync

A fast CLI tool to sync node and service definitions to Consul Catalog using Transaction API.

## Overview

This tool bridges the gap between your existing infrastructure definitions and Consul's API structure. Instead of forcing you to adapt your YAML structures to match Consul's specific format, it allows you to:

1. **Keep your existing YAML structure** - Define nodes and services in a format that makes sense for your organization
2. **Use mapping rules** - Transform your structure to Consul's format using simple templating rules
3. **Apply changes quickly** - Direct use of Consul's Transaction API for fast, atomic updates

### Why is this needed?

When managing infrastructure with Consul, you often face a dilemma:

- Your YAML data should reflect your infrastructure's logical organization, not Consul's API structure
- Different teams may have different data models that work best for their domains
- Consul requires a specific JSON format for its Catalog API
- Being forced to adopt Consul's data structure pollutes your configuration with implementation details

This tool solves that by providing a transformation layer. You define the mapping once, and then your team can continue using familiar YAML structures while still leveraging Consul's powerful service discovery features.

### Example transformation

Your YAML structure:
```yaml
web-server-01:
  ip: 10.0.1.5
  type: web
  location: rack-3
```

Gets transformed to Consul's format:
```json
{
  "Node": {
    "Node": "web-server-01",
    "Address": "10.0.1.5",
    "Meta": {
      "type": "web",
      "location": "rack-3"
    }
  }
}
```

The mapping rules define how this transformation happens, keeping your infrastructure definitions independent from Consul's implementation details.

## Features

- **Fast**: Direct use of Consul Transaction API for bulk operations
- **Flexible**: Supports single file or directory of YAML files
- **Safe**: Dry-run mode to preview changes
- **Debuggable**: Output JSON payload for inspection

## Installation

```bash
$ go install github.com/example/consul-catalog-sync@latest
```

## Usage

### Basic usage

Sync from single file (uses defaults: dc1, http://127.0.0.1:8500)

```bash
$ consul-catalog-sync -vars nodes.yaml -mapping mapping.yaml
```

Sync from directory with specific datacenter

```bash
$ consul-catalog-sync -vars vars/ -mapping mapping.yaml -datacenter prod
```

Use custom Consul address

```bash
$ consul-catalog-sync -vars vars/ -mapping mapping.yaml -consul-addr http://consul.example.com:8500
```

Dry run to preview changes

```bash
$ consul-catalog-sync -vars vars/ -mapping mapping.yaml -dry-run
```

Output JSON payload for debugging

```bash
$ consul-catalog-sync -vars vars/ -mapping mapping.yaml -payload | jq '.'
```

### Required flags

- `-vars PATH`: Path to vars file or directory containing YAML files
- `-mapping FILE`: Path to mapping rules file

### Optional flags

- `-datacenter DC`: Target datacenter (default: `dc1`)
- `-consul-addr URL`: Consul HTTP address (default: `http://127.0.0.1:8500`)
- `-dry-run`: Show operations without executing
- `-verbose`: Verbose output
- `-payload`: Output JSON payload (NDJSON format)
- `-help`: Show help message
- `-version`: Show version

## File formats

See `examples/` directory for vars and mapping file formats.

## Examples

### Single file

```bash
$ cd examples/simple
$ consul-catalog-sync -vars nodes.yaml -mapping mapping.yaml -dry-run
```

### Directory structure

```
examples/structured/
├── mapping.yaml
└── vars/
    ├── group1/
    │   └── items.yaml
    ├── group2/
    │   └── items.yaml
    └── group3/
        └── data.yaml
```

Sync all vars from directory

```bash
$ consul-catalog-sync -vars examples/structured/vars -mapping examples/structured/mapping.yaml -dry-run
```

Sync specific group only

```bash
$ consul-catalog-sync -vars examples/structured/vars/group1 -mapping examples/structured/mapping.yaml -dry-run
```

View the JSON payload

```bash
$ consul-catalog-sync -vars examples/structured/vars -mapping examples/structured/mapping.yaml -payload | jq '.'
```

## License

This project is licensed under the MIT License - see the [LICENSE](https://opensource.org/license/mit) for details.
