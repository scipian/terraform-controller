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

package terraform

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Configmap", func() {

	testBackend := `
terraform {
	backend "s3" {
		bucket               = "test-backend"
		key                  = "terraform.tfstate"
		region               = "us-west-2"
		dynamodb_table       = "test-locking"
		workspace_key_prefix = "namespace"
		access_key           = "test-key"
		secret_key           = "test-secret"
	}
}
	`

	testWorkspaceBackend := terraformv1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: terraformv1.WorkspaceSpec{
			Region: "us-west-2",
			TfVars: map[string]string{"foo": "bar"},
		},
	}

	ws := &testWorkspaceBackend

	variableMap := map[string]string{
		"network_workspace_namespace": "namespace",
		"state_bucket_name":           "test-backend",
		"access_key":                  "test-key",
		"secret_key":                  "test-secret",
	}

	Context("Format Terraform Backend", func() {
		It("Should not be empty", func() {
			Expect(formatBackendTerraform("test-backend", "test-locking", "test-key", "test-secret", ws)).NotTo(BeEmpty())
		})
		It("Should match testBackend", func() {
			Expect(formatBackendTerraform("test-backend", "test-locking", "test-key", "test-secret", ws)).Should(Equal(testBackend))
		})
	})

	Context("Format Terraform Variables", func() {
		It("Should not be empty", func() {
			Expect(formatTerraformVars(variableMap, ws)).NotTo(BeEmpty())
		})
	})

	Context("Create configmap", func() {
		It("Should contain expected values", func() {
			key := types.NamespacedName{Namespace: "bar", Name: "foo"}
			configMap := CreateConfigMap(key, "test-key", "test-secret", ws)
			Expect(configMap.Name).Should(Equal("foo"))
			Expect(configMap.Namespace).Should(Equal("bar"))
			Expect(configMap.Data).Should(HaveKey("backend-tf"))
			Expect(configMap.Data).Should(HaveKey("terraform-tfvars"))
		})
	})
})
