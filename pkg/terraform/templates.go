package terraform

const (
	// RemoteState is a template for Scipian's s3 backend configuration
	// The order of these formated string inputs are: referenced network workspace name, current resource workspace name, and namespace.
	// Note the "%%s" which is a literal percent sign, which will be populated by Terraform later with the var.network_workspace_name value.
	RemoteState = `
locals {
	aws_region          = "${data.aws_region.current.name}"
	state_bucket_region = "${local.aws_region == "cn-north-1" || local.aws_region == "cn-northwest-1" ? "cn-north-1" : "us-west-2" }"
}

data "aws_region" "current" {}

data "terraform_remote_state" "%s" {
	backend = "s3"
  
	config {
	  bucket = "scipian-backend"
	  key    = "%s"
	  key    = "${format("env:/%s/%%s/terraform.tfstate", var.network_workspace_name)}"
	  region = local.state_bucket_region
	}
  }
	`
	// BackendTemplate is the template for a Terraform Backend
	BackendTemplate = `
terraform {
	backend "%s" {
		bucket         = "%s"
		key            = "%s"
		region         = "%s"
		dynamodb_table = "%s"
	}
}
	`
)
