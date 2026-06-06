resource "dnswiz_record" "www_a" {
  zone_id = dnswiz_zone.example.id
  name    = "www"
  type    = "A"
  ttl     = 300
  value   = "192.0.2.1"
}

resource "dnswiz_record" "apex_mx" {
  zone_id  = dnswiz_zone.example.id
  name     = "@"
  type     = "MX"
  ttl      = 3600
  value    = "mail.example.com"
  priority = 10
}

resource "dnswiz_record" "spf" {
  zone_id = dnswiz_zone.example.id
  name    = "@"
  type    = "TXT"
  ttl     = 3600
  value   = "v=spf1 -all"
}

resource "dnswiz_record" "geo_www" {
  zone_id = dnswiz_zone.example.id
  name    = "www"
  type    = "POOL"
  pool_id = dnswiz_pool.web.id
}
