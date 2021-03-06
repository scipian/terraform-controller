package terraform

import (
	"fmt"
	"os"

	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;pods;pods/volumes,verbs=get;list;watch;create;update;patch;delete

// CreateConfigMap creates a Kubernetes Configmap with variables that the Terraform Job will reference
func CreateConfigMap(key types.NamespacedName, accessKey string, secretKey string, ws *terraformv1.Workspace) *corev1.ConfigMap {
	scipianBucket := os.Getenv("SCIPIAN_STATE_BUCKET")
	scipianStateLocking := os.Getenv("SCIPIAN_STATE_LOCKING")
	//TODO(NL): Add error handling here if ENV's are empty

	backendVariableMap := map[string]string{
		"network_workspace_namespace": ws.Namespace,
		"state_bucket_name":           scipianBucket,
		"access_key":                  accessKey,
		"secret_key":                  secretKey,
	}

	backendTF := formatBackendTerraform(scipianBucket, scipianStateLocking, accessKey, secretKey, ws)
	tfVars := formatTerraformVars(backendVariableMap, ws)

	configMapData := make(map[string]string)
	configMapData["backend-tf"] = backendTF
	configMapData["terraform-tfvars"] = tfVars

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    make(map[string]string),
		},
		Data: configMapData,
	}
}

func formatBackendTerraform(bucket string, stateLocking string, accessKey string, secretKey string, ws *terraformv1.Workspace) string {
	var region string

	if stateLocking == "" {
		stateLocking = fmt.Sprintf("%s-locking", bucket)
	}

	if ws.Spec.Region == "cn-north-1" || ws.Spec.Region == "cn-northwest-1" {
		region = "cn-north-1"
	} else {
		region = "us-west-2"
	}
	backend := fmt.Sprintf(BackendTemplate, bucket, region, stateLocking, ws.Namespace, accessKey, secretKey)
	return backend
}

func formatTerraformVars(variableMap map[string]string, ws *terraformv1.Workspace) string {
	var terraformVariables, providedVariables, backendVariables string

	// range over backend variables
	for k, v := range variableMap {
		backendVariables = fmt.Sprintf(`%s = "%s"`, k, v)
		terraformVariables = terraformVariables + backendVariables + "\n"
	}

	// range over provided variables
	for k, v := range ws.Spec.TfVars {
		providedVariables = fmt.Sprintf(`%s = "%s"`, k, v)
		terraformVariables = terraformVariables + providedVariables + "\n"
	}
	return terraformVariables
}
