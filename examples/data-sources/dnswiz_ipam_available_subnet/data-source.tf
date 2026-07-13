# Find the next free /24 inside a block (read-time peek).
data "dnswiz_ipam_available_subnet" "next" {
  block_id      = dnswiz_ipam_block.east.id
  prefix_length = 24
}

output "next_free_subnet" {
  value = data.dnswiz_ipam_available_subnet.next.cidr
}
