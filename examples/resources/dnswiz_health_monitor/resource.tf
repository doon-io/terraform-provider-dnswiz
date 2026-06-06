resource "dnswiz_health_monitor" "https" {
  name             = "https-root"
  kind             = "https"
  path             = "/healthz"
  expected_status  = 200
  interval_seconds = 10
  timeout_seconds  = 5
  healthy_after    = 2
  unhealthy_after  = 3
}
