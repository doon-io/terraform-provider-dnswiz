# Blocks self-organize by CIDR containment — declare the supernet and the
# prefixes it covers nest underneath it automatically. No parent to wire.
resource "dnswiz_ipam_block" "corp" {
  cidr   = "10.0.0.0/8"
  name   = "corp"
  origin = "rfc1918"
}

resource "dnswiz_ipam_block" "east" {
  cidr = "10.1.0.0/16"
  name = "us-east"
  # dnswiz places this under dnswiz_ipam_block.corp automatically.
}
