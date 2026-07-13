# Explicit host.
resource "dnswiz_ipam_ip_address" "gw" {
  network_id = dnswiz_ipam_network.app.id
  address    = "10.1.6.1"
  hostname   = "gw01"
  tags       = ["role:gateway"]
}

# Next free host in the network (concurrency-safe — allocated on create).
resource "dnswiz_ipam_ip_address" "vip" {
  network_id = dnswiz_ipam_network.app.id
  hostname   = "vip"
}
