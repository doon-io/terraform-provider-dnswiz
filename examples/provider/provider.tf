terraform {
  required_providers {
    dnswiz = {
      source  = "doon-io/dnswiz"
      version = "~> 0.1"
    }
  }
}

# api_key can also come from the DNSWIZ_API_KEY environment variable.
provider "dnswiz" {
  api_key = var.dnswiz_api_key
}

variable "dnswiz_api_key" {
  description = "API key for the dnswiz account."
  type        = string
  sensitive   = true
}
