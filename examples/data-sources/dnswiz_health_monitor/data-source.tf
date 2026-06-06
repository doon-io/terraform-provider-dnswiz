data "dnswiz_health_monitor" "https_preset" {
  name = "HTTPS"
}

resource "dnswiz_pool" "web" {
  name              = "web-eu"
  health_monitor_id = data.dnswiz_health_monitor.https_preset.id
}
