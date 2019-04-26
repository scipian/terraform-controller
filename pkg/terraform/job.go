package terraform

import (
	"context"
	"fmt"

	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	falseVal     = false
	trueVal      = true
	backOffLimit int32
)

type configMapReader struct {
	client.Client
}

// StartJob starts a Kubernetes Job that runs Terraform on a given set of files
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
func StartJob(name string, namespace string, tfCmd string, ws *terraformv1alpha1.Workspace) *batchv1.Job {
	terraformCommand := fmt.Sprintf(tfCmd, ws.Spec.WorkingDir, ws.Name)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    make(map[string]string),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: make(map[string]string),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:       name,
							Image:      ws.Spec.Image,
							Command:    []string{"/bin/ash"},
							Args:       []string{"-c", terraformCommand},
							WorkingDir: ws.Spec.WorkingDir,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &falseVal,
							},
							ImagePullPolicy: corev1.PullPolicy(corev1.PullIfNotPresent),
							Env:             getEnv(ws),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-map",
									MountPath: "/opt/meta",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "config-map",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: name,
									},
									Items: getConfigMapItems(ws),
									// Items: []corev1.KeyToPath{
									// 	{
									// 		Key:  "terraform-tfvars",
									// 		Path: "terraform.tfvars",
									// 	},
									// 	{
									// 		Key:  "backend-tf",
									// 		Path: "backend.tf",
									// 	},
									// 	{
									// 		Key:  "remote-state",
									// 		Path: "remote-state.tf",
									// 	},
									// },
									Optional: &trueVal,
								},
							},
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{},
				},
			},
		},
	}
}

func getEnv(ws *terraformv1alpha1.Workspace) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ws.Spec.Secret,
					},
					Key: "aws_access_key_id",
				},
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ws.Spec.Secret,
					},
					Key: "aws_secret_access_key",
				},
			},
		},
	}
	for k, v := range ws.Spec.EnvVars {
		env = append(env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return env
}

func getConfigMapItems(ws *terraformv1alpha1.Workspace) []corev1.KeyToPath {
	c := &configMapReader{}
	configMap := c.getConfigMap(ws)
	items := []corev1.KeyToPath{}

	for k, v := range configMap.Data {
		items = append(items, corev1.KeyToPath{
			Key:  k,
			Path: v,
		})
	}
	return items
}

func (c *configMapReader) getConfigMap(ws *terraformv1alpha1.Workspace) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	// TODO(NL): Handle possible error returned by Get
	c.Get(context.TODO(), types.NamespacedName{Name: ws.Name, Namespace: ws.Namespace}, cm)
	return cm
}
