resource "dnswiz_endpoint" "web_a" {
  name              = "web-frankfurt"
  value             = "203.0.113.10"
  host              = "203.0.113.10"
  port              = 443
  health_monitor_id = dnswiz_health_monitor.https.id
}
