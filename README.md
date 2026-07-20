# Terraform Provider for Pingvera

The official Terraform provider for [Pingvera](https://pingvera.ru) — a website &
server monitoring platform (multi-region uptime checks + host/container metrics via
an agent, incidents, alerts, and status pages).

Manage your monitors and status pages as code, in the same `plan`/`apply` workflow
as the rest of your infrastructure.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
  (>= 1.5 to use the `import` blocks emitted by `pingvera terraform generate`)
- A Pingvera account and an API token with `write:*` scopes
  ([app.pingvera.ru](https://app.pingvera.ru) → Settings → API tokens)

## Usage

```hcl
terraform {
  required_providers {
    pingvera = {
      source  = "pingvera/pingvera"
      version = "~> 0.1"
    }
  }
}

provider "pingvera" {
  # endpoint = "https://app.pingvera.ru"  # default; use https://app.pingvera.com for the EN instance
  # token    = "pv_..."                   # better via the PINGVERA_TOKEN env var
}

resource "pingvera_monitor" "website" {
  type   = "http"
  name   = "Main website"
  target = "https://example.com"
}

resource "pingvera_status_page" "public" {
  slug     = "acme"
  title    = "Acme Status"
  theme    = "dark"
  monitors = [pingvera_monitor.website.id]
}
```

```bash
export PINGVERA_TOKEN=pv_xxxxxxxx
terraform init
terraform plan
terraform apply
```

## Provider configuration

| Argument   | Env var          | Required | Default                   |
|------------|------------------|----------|---------------------------|
| `endpoint` | `PINGVERA_URL`   | no       | `https://app.pingvera.ru` |
| `token`    | `PINGVERA_TOKEN` | yes      | —                         |

Never commit the token to `.tf` files — pass it via `PINGVERA_TOKEN`.

## Resources

### `pingvera_monitor`

| Attribute             | Type                    | Notes                                                                                     |
|-----------------------|-------------------------|-------------------------------------------------------------------------------------------|
| `type`                | string, required        | `http`, `tcp`, `dns`, `tls`, `wp`, `domain`, `links`, `heartbeat`, … (forces replacement) |
| `name`                | string, required        |                                                                                           |
| `target`              | string, required        | URL/host being checked (forces replacement)                                               |
| `interval_s`          | number, optional        | check interval, seconds                                                                   |
| `fail_threshold`      | number, optional        | consecutive failures before an incident                                                   |
| `degraded_latency_ms` | number, optional        | latency threshold for the "degraded" state                                                |
| `enabled`             | bool, optional          |                                                                                           |
| `tags`                | set(string), optional   |                                                                                           |
| `config`              | string (JSON), optional | type-specific settings                                                                    |
| `id`                  | string, computed        | monitor public_id                                                                         |

### `pingvera_status_page`

| Attribute     | Type                  | Notes                                            |
|---------------|-----------------------|--------------------------------------------------|
| `slug`        | string, required      | URL slug of the public page                      |
| `title`       | string, optional      |                                                  |
| `theme`       | string, optional      | `""` (default), `light`, `dark`, `midnight`      |
| `brand_color` | string, optional      | `#rrggbb`                                         |
| `monitors`    | set(string), optional | monitor public_ids (use `pingvera_monitor.x.id`) |
| `hosts`       | set(string), optional | agent/server public_ids                          |
| `logo_url`, `header_md`, `footer_md`, `embed_domains`, `timezone`, `custom_domain` | string, optional | |
| `id`          | string, computed      | status page public_id                            |

Full docs: [`docs/resources/`](docs/resources/). More examples: [`examples/`](examples/).

## Bootstrap from an existing account

Already have monitors and status pages in the dashboard? The Pingvera CLI generates
ready-to-apply HCL — with `import` blocks pre-wired, so the first `terraform plan`
shows **zero drift**:

```bash
export PINGVERA_TOKEN=pv_xxxxxxxx
pingvera terraform generate > pingvera.tf
terraform plan   # No changes.
```

Get the CLI:

```bash
curl -fsSL https://app.pingvera.ru/dl/pingvera-cli-linux-amd64 -o pingvera && chmod +x pingvera
# macOS: pingvera-cli-darwin-arm64 / -amd64 · Windows: pingvera-cli-windows-amd64.exe
```

## Local development

Until the provider is published to the Terraform Registry, point Terraform at a
locally built binary via `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "pingvera/pingvera" = "/path/to/bin"
  }
  direct {}
}
```

```bash
go build -o bin/terraform-provider-pingvera .
go test ./...
go vet ./...
```

## License

[MPL-2.0](LICENSE).
