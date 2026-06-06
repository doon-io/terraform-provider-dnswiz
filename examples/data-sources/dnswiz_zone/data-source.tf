data "dnswiz_zone" "existing" {
  name = "example.com"
}

# Use the looked-up zone id to manage records in a zone you didn't
# create with Terraform.
resource "dnswiz_record" "www" {
  zone_id = data.dnswiz_zone.existing.id
  name    = "www"
  type    = "A"
  ttl     = 300
  value   = "192.0.2.10"
}
