# Contributing

Thanks for your interest in improving the dnswiz Terraform provider.

## Reporting issues

Please file bugs and feature requests in the
[issue tracker](https://github.com/doon-io/terraform-provider-dnswiz/issues).
Include the provider version, Terraform version, and a minimal HCL
snippet that reproduces the problem.

## Building from source

```sh
git clone https://github.com/doon-io/terraform-provider-dnswiz
cd terraform-provider-dnswiz
make build
```

The output binary is `./terraform-provider-dnswiz`.

## Trying a local build from a real Terraform config

Add a `dev_overrides` block to your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "doon-io/dnswiz" = "/absolute/path/to/your/checkout"
  }
  direct {}
}
```

Then `go install .` and any `terraform plan` picks up your local
binary without needing `terraform init`. Remove the block when you go
back to the released provider.

## Running tests

```sh
# Unit tests
make test

# Acceptance tests against a live dnswiz account.
# These create and delete real resources, so use a disposable account.
DNSWIZ_API_KEY=your-key make testacc
```

## Pull requests

- Run `make fmt vet test` before opening a PR.
- Add or update an example under `examples/` when changing a resource
  schema.
- For new resources, include acceptance tests under
  `internal/provider/`.
