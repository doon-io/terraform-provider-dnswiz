# Three ways to place a network — pick one.

# 1. Explicit CIDR inside a block.
resource "dnswiz_ipam_network" "db" {
  cidr            = "10.1.5.0/24"
  parent_block_id = dnswiz_ipam_block.east.id
  name            = "db-tier"
  gateway_ip      = "10.1.5.1"
}

# 2. Next free /24 from a specific block.
resource "dnswiz_ipam_network" "app" {
  parent_block_id = dnswiz_ipam_block.east.id
  prefix_length   = 24
  name            = "app-tier"
}

# 3. Next free /24 from ANY block tagged region:eu-central-1 (a prefix pool).
#    Lowest-CIDR block with room wins; fills predictably.
resource "dnswiz_ipam_network" "eu" {
  parent_tags   = ["region:eu-central-1"]
  prefix_length = 24
  name          = "eu-app"
  tags          = ["env:prod", "team:platform"]
}
