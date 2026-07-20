terraform {
  backend "http" {
    address        = "http://localhost:8080/state/test-workspace"
    lock_address   = "http://localhost:8080/state/test-workspace"
    unlock_address = "http://localhost:8080/state/test-workspace"
    lock_method    = "LOCK"
    unlock_method  = "UNLOCK"
    username       = "terraform"
    password       = "secret-password"
  }
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = ">= 2.0.0"
    }
  }
}

resource "local_file" "test" {
  filename = "${path.module}/test.txt"
  content  = "Hello, Terraform HTTP Backend!"
}
