package terraform

import (
	"os"
	"testing"

	"github.com/onsi/gomega"
	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Set up desired Workspace
var testWorkspaceBackend = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "name",
		Namespace: "namespace",
	},
	Spec: terraformv1alpha1.WorkspaceSpec{
		Region:        "us-west-2",
		Bucket:        "test-backend",
		DynamoDBTable: "test-locking",
		TfVars:        map[string]string{"foo": "bar"},
	},
}

// Set up desired backend blob
var testBackend = `
terraform {
	backend "s3" {
		bucket               = "test-backend"
		key                  = "terraform.tfstate"
		region               = "us-west-2"
		dynamodb_table       = "test-locking"
		workspace_key_prefix = "namespace"
	}
}
	`

// Set up desired tfVars blob
var testTfVars = `network_workspace_namespace = "namespace"
state_bucket_name = "test-backend"
foo = "bar"
`

// Set up desired ConfigMap
var testConfigMap = corev1.ConfigMap{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "foo",
		Namespace: "bar",
		Labels:    make(map[string]string),
	},
	Data: map[string]string{"backend-tf": testBackend, "terraform-tfvars": testTfVars},
}

func TestCreateConfigMap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ws := &testWorkspaceBackend
	cm := &testConfigMap

	tempBucket := os.Getenv("SCIPIAN_STATE_BUCKET")
	tempLocking := os.Getenv("SCIPIAN_STATE_LOCKING")
	os.Setenv("SCIPIAN_STATE_BUCKET", "test-backend")
	os.Setenv("SCIPIAN_STATE_LOCKING", "test-locking")

	defer os.Setenv("SCIPIAN_STATE_BUCKET", tempBucket)
	defer os.Setenv("SCIPIAN_STATE_LOCKING", tempLocking)

	// Test formatBackendTerraform function
	g.Expect(formatBackendTerraform(ws)).Should(gomega.Equal(testBackend))
	g.Expect(formatBackendTerraform(ws)).NotTo(gomega.BeEmpty())

	// Test formatTerraformVars function
	g.Expect(formatTerraformVars(ws)).Should(gomega.Equal(testTfVars))
	g.Expect(formatTerraformVars(ws)).NotTo(gomega.BeEmpty())

	// Test CreatConfigMap function
	configMap := CreateConfigMap("foo", "bar", ws)
	g.Expect(configMap).Should(gomega.Equal(cm))
}
