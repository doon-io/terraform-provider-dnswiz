resource "dnswiz_notification_channel" "oncall" {
  name   = "oncall-webhook"
  kind   = "webhook"
  target = "https://hooks.example.com/dnswiz"
  events = [
    "endpoint.down",
    "endpoint.up",
    "hijack.detected",
    "cert.expiring",
  ]
}

# The signing secret is only returned once at create time. Save it
# wherever your webhook receiver verifies HMAC signatures.
output "channel_secret" {
  value     = dnswiz_notification_channel.oncall.secret
  sensitive = true
}
