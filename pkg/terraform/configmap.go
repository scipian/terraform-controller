package terraform

import (
	"fmt"
	"os"

	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateConfigMap creates a Kubernetes Configmap with variables that the Terraform Job will reference
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;pods;pods/volumes,verbs=get;list;watch;create;update;patch;delete
func CreateConfigMap(name string, namespace string, ws *terraformv1alpha1.Workspace) *corev1.ConfigMap {
	backendTF := formatBackendTerraform(ws)
	tfVars := formatTerraformVars(ws)

	configMapData := make(map[string]string)
	configMapData["backend-tf"] = backendTF
	configMapData["terraform-tfvars"] = tfVars

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    make(map[string]string),
		},
		Data: configMapData,
	}
}

func formatBackendTerraform(ws *terraformv1alpha1.Workspace) string {
	var region string
	scipianBucket := os.Getenv("SCIPIAN_STATE_BUCKET")
	scipianStateLocking := os.Getenv("SCIPIAN_STATE_LOCKING")

	if scipianStateLocking == "" {
		scipianStateLocking = fmt.Sprintf("%s-locking", scipianBucket)
	}

	if ws.Spec.Region == "cn-north-1" || ws.Spec.Region == "cn-northwest-1" {
		region = "cn-north-1"
	} else {
		region = "us-west-2"
	}
	backend := fmt.Sprintf(BackendTemplate, scipianBucket, region, scipianStateLocking, ws.Namespace)
	return backend
}

func formatTerraformVars(ws *terraformv1alpha1.Workspace) string {
	var terraformVariables, variable string
	var namespaceVariable = fmt.Sprintf(`network_workspace_namespace = "%s"`, ws.Namespace)
	terraformVariables = terraformVariables + namespaceVariable + "\n"

	for k, v := range ws.Spec.TfVars {
		variable = fmt.Sprintf(`%s = "%s"`, k, v)
		terraformVariables = terraformVariables + variable + "\n"
	}
	return terraformVariables
}
