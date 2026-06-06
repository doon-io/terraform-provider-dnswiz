# Issue a Let's Encrypt cert for api.example.com. dnswiz solves the
# DNS-01 challenge against the matching zone (managed elsewhere in
# this config or in the dashboard), so no extra wiring is needed.
resource "dnswiz_cert" "api" {
  names = ["api.example.com"]
}

# Multi-SAN cert covering both the apex and a wildcard. Useful when
# you want the same cert on the marketing site and every subdomain.
resource "dnswiz_cert" "web" {
  names = [
    "example.com",
    "*.example.com",
  ]

  # Renew when fewer than 21 days remain. Default is 30. Set to 0 to
  # disable auto-renew and rotate manually with `terraform apply -replace=...`.
  min_days_remaining = 21
}

# Wire the issued material into an AWS ACM import, an nginx config,
# or anything else that wants PEM. private_key_pem is a sensitive
# value, so make sure your Terraform state backend is encrypted at rest.
output "api_fullchain" {
  value     = dnswiz_cert.api.fullchain_pem
  sensitive = false
}

output "api_private_key" {
  value     = dnswiz_cert.api.private_key_pem
  sensitive = true
}
