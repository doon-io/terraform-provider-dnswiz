resource "dnswiz_zone_policy" "firewall" {
  zone_id = dnswiz_zone.example.id
  kind    = "query_firewall"
  enabled = true
  config_json = jsonencode({
    allow_sources   = ["10.0.0.0/8", "192.168.0.0/16"]
    deny_countries  = ["RU", "KP"]
    refuse_qtypes   = ["ANY"]
    rate_limit_qps  = 50
  })
}

resource "dnswiz_zone_policy" "hijack" {
  zone_id = dnswiz_zone.example.id
  kind    = "hijack_monitor"
  enabled = true
}
