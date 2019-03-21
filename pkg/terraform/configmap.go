package terraform

import (
	"fmt"

	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	backendTemplate = `
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
	backendType := ws.Spec.Backend.Type
	bucket := ws.Spec.Backend.Bucket
	key := ws.Spec.Backend.Key
	region := ws.Spec.Backend.Region
	dbTable := ws.Spec.Backend.DynamoDBTable

	backend := fmt.Sprintf(backendTemplate, backendType, bucket, key, region, dbTable)
	return backend
}

func formatTerraformVars(ws *terraformv1alpha1.Workspace) string {
	var terraformVariables, variable string

	for k, v := range ws.Spec.TfVars {
		variable = fmt.Sprintf(`%s = "%s"`, k, v)
		terraformVariables = terraformVariables + variable + "\n"
	}
	return terraformVariables
}
