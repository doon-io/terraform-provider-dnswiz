resource "dnswiz_pool" "web" {
  name              = "web-eu"
  description       = "EU-region web servers"
  health_monitor_id = dnswiz_health_monitor.https.id
  selection_method  = "weighted"
}

resource "dnswiz_pool_member" "web_a" {
  pool_id     = dnswiz_pool.web.id
  endpoint_id = dnswiz_endpoint.web_a.id
  weight      = 100
}

resource "dnswiz_pool_member" "web_b" {
  pool_id     = dnswiz_pool.web.id
  endpoint_id = dnswiz_endpoint.web_b.id
  weight      = 50
}
