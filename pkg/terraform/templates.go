package terraform

const (
	// BackendTemplate is a template for a terraform backend
	BackendTemplate = `
terraform {
	backend "s3" {
		bucket               = "%s"
		key                  = "terraform.tfstate"
		region               = "%s"
		dynamodb_table       = "%s"
		workspace_key_prefix = "%s"
		access_key           = "%s"
		secret_key           = "%s"
	}
}
	`
)
