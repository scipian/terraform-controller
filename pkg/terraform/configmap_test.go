package terraform

import (
	"testing"

	"github.com/onsi/gomega"
	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Set up desired Workspace
var testWorkspaceBackend = terraformv1alpha1.Workspace{
	Spec: terraformv1alpha1.WorkspaceSpec{
		Backend: terraformv1alpha1.WorkspaceBackend{
			Type:          "fakeType",
			Bucket:        "fakeBucket",
			Key:           "fakeKey",
			Region:        "fakeRegion",
			DynamoDBTable: "fakeDBTable",
		},
		TfVars: map[string]string{"foo": "bar"},
	},
}

// Set up desired backend blob
var testBackend = `
terraform {
	backend "fakeType" {
		bucket         = "fakeBucket"
		key            = "fakeKey"
		region         = "fakeRegion"
		dynamodb_table = "fakeDBTable"
	}
}
	`

// Set up desired tfVars blob
var testTfVars = `foo = "bar"
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
