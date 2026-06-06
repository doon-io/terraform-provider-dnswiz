resource "dnswiz_zone" "example" {
  name        = "example.com"
  default_ttl = 300
}
