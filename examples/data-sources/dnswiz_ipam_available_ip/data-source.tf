data "dnswiz_ipam_available_ip" "next" {
  network_id = dnswiz_ipam_network.app.id
}

output "next_free_ip" {
  value = data.dnswiz_ipam_available_ip.next.address
}
