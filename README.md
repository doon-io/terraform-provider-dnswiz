# Terraform Provider for dnswiz

Manage [dnswiz](https://dnswiz.app) zones, records, GSLB pools, and policies as code.

## Status

`v0`, early development. Resource coverage is still growing. See the
[CHANGELOG](./CHANGELOG.md) for what each release adds.

## Requirements

- Terraform 1.6 or newer
- Go 1.23 or newer (only if building from source)
- A dnswiz account and an API key from `Settings → API keys` in the console

## Quick start

```hcl
terraform {
  required_providers {
    dnswiz = {
      source  = "doon-io/dnswiz"
      version = "~> 0.1"
    }
  }
}

provider "dnswiz" {
  api_key = var.dnswiz_api_key
}

resource "dnswiz_zone" "example" {
  name = "example.com"
}
```

## Provider configuration

| Argument   | Env var          | Required | Description                                                          |
| ---------- | ---------------- | -------- | -------------------------------------------------------------------- |
| `endpoint` | `DNSWIZ_ENDPOINT`| no       | Base URL of the dnswiz API. Defaults to `https://console.dnswiz.app`. Set this if you self-host. |
| `api_key`  | `DNSWIZ_API_KEY` | yes      | API key from the dnswiz console under Settings, API keys.            |

## Resources

| Name                 | Status      |
| -------------------- | ----------- |
| `dnswiz_zone`        | implemented |
| `dnswiz_record`      | planned     |
| `dnswiz_pool`        | planned     |
| `dnswiz_pool_member` | planned     |
| `dnswiz_endpoint`    | planned     |

## Local development

```sh
git clone https://github.com/doon-io/terraform-provider-dnswiz
cd terraform-provider-dnswiz

# Build the provider binary
make build

# Run unit tests
make test

# Run acceptance tests against a real dnswiz API.
# These create and destroy actual resources on the target account, so
# point them at a sandbox or disposable tenant.
DNSWIZ_ENDPOINT=http://localhost:8787 \
DNSWIZ_API_KEY=dwk_... \
  make testacc
```

To try an unreleased local build from a real Terraform config, add a
`dev_overrides` block to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "doon-io/dnswiz" = "/Users/you/code/terraform-provider-dnswiz"
  }
  direct {}
}
```

Then `go install .` and any `terraform plan` picks up your local binary
without needing `terraform init`.

## License

[MPL-2.0](./LICENSE)
