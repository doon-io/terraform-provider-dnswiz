resource "dnswiz_zone" "example" {
  name         = "example.com"
  default_ttl  = 300
  soa_rname    = "hostmaster@example.com"
  negative_ttl = 60
}
