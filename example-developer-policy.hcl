path "secret/prd/namespace/*" {
  capabilities = ["create", "list", "update"]

  allowed_parameters = {
    "secret" = []
  }
}

path "secret/stg/namespace/*" {
  capabilities = ["create", "list", "update"]

  allowed_parameters = {
    "secret" = []
  }
}

path "secret/test/namespace/*" {
  capabilities = ["create", "list", "update", "read"]

  allowed_parameters = {
    "secret" = []
  }
}
