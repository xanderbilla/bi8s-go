// TFLint configuration shared across modules and envs.
// AWS plugin gives provider-aware rules (deprecated arg detection,
// invalid instance types, missing tags, etc.).

config {
  call_module_type = "all"
}

plugin "aws" {
  enabled = true
  version = "0.32.0"
  source  = "github.com/terraform-linters/tflint-ruleset-aws"
}

plugin "terraform" {
  enabled = true
  preset  = "recommended"
}
