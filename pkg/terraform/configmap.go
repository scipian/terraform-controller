package terraform

import (
	"fmt"

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

	if ws.Spec.RemoteState != "" {
		// This workspace is referencing state from a Network workspace
		remoteState := formatRemoteState(ws)
		configMapData["remote-state"] = remoteState
	}

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

	backend := fmt.Sprintf(BackendTemplate, backendType, bucket, key, region, dbTable)
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

func formatRemoteState(ws *terraformv1alpha1.Workspace) string {
	remoteState := fmt.Sprintf(RemoteState, ws.Spec.RemoteState, ws.Name, ws.Namespace)
	return remoteState
}
