/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// WorkspaceFinalizerName is the name of the finalizer assigned to Workspace objects
	WorkspaceFinalizerName = "workspace.finalizer.scipian.io"

	// ScipianIAMSecretName is the Secret name for the Scipian IAM credentials
	ScipianIAMSecretName = "scipian-aws-iam-creds"

	// ScipianNamespace is the Namespace the ScipianIAMSecretName exists
	ScipianNamespace = "scipian"

	// AccessKey is the AWS_ACCESS_KEY_ID name for the Scipian AWS IAM creds stored in the ScipianIAMSecretName
	AccessKey = "aws_access_key_id"

	// SecretKey is the AWS_SECRET_ACCESS_KEY name for the Scipian AWS IAM creds stored in the ScipianIAMSecretName
	SecretKey = "aws_secret_access_key"

	// TFWorkspaceNew is the Terraform command for creating a new Terraform Workspace
	TFWorkspaceNew = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace new %s"

	// TFWorkspaceDelete is the Terraform command for deleting an existing Terraform Workspace
	TFWorkspaceDelete = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace delete -force %s"

	// TFPlan is the Terraform command for initializing, selecting a workspace, planning, and applying
	TFPlan = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace select %s && terraform plan -input=false -out=plan.bin && terraform apply -input=false plan.bin"

	// TFDestroy is the Terraform command for running Terraform destroy on resources
	TFDestroy = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace select %s && terraform destroy -auto-approve"
)

// Object is used as a helper interface when passing Kubernetes resources
// between methods.
// All Kubernetes resources should implement both of these interfaces
type Object interface {
	runtime.Object
	metav1.Object
}
