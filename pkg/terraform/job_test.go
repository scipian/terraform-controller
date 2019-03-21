package terraform

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"

	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	jobName           = "test-job"
	jobNamespace      = "test-namespace"
	secretName        = "test-secret"
	image             = "test-image"
	workDir           = "test-working-dir"
	tfCommandTemplate = "foo %s %s"
	desiredTfCommand  = fmt.Sprintf(tfCommandTemplate, workDir, jobName)
)

// Set up desired Secret object
var desiredTestSecret = corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      secretName,
		Namespace: jobNamespace,
	},
	Type:       "Opaque",
	StringData: map[string]string{"aws_access_key_id": "fake-id", "aws_secret_access_key": "fake-key"},
}

// Set up desired corev1.EnvVar object
var desiredTestEnvVar = []corev1.EnvVar{
	{
		Name: "AWS_ACCESS_KEY_ID",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
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
					Name: secretName,
				},
				Key: "aws_secret_access_key",
			},
		},
	},
	{
		Name:  "FOO",
		Value: "foo",
	},
}

// Set up desired Workspace
var desiredTestWorkspaceForJob = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name: jobName,
	},
	Spec: terraformv1alpha1.WorkspaceSpec{
		EnvVars:    map[string]string{"FOO": "foo"},
		Secret:     secretName,
		Image:      image,
		WorkingDir: workDir,
	},
}

// Set up desired ConfigMap
var desiredTestJobConfigMap = corev1.ConfigMap{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      jobName,
		Namespace: jobNamespace,
		Labels:    make(map[string]string),
	},
	Data: map[string]string{"backend-tf": "backend-string", "terraform-tfvars": "tf-vars-string"},
}

// Set up desired Job object
var desiredJobObject = batchv1.Job{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Job",
		APIVersion: "batch/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      jobName,
		Namespace: jobNamespace,
		Labels:    make(map[string]string),
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: &backOffLimit,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:   jobName,
				Labels: make(map[string]string),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:       jobName,
						Image:      image,
						Command:    []string{"/bin/ash"},
						Args:       []string{"-c", desiredTfCommand},
						WorkingDir: workDir,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &falseVal,
						},
						ImagePullPolicy: corev1.PullPolicy(corev1.PullIfNotPresent),
						Env:             desiredTestEnvVar,
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
									Name: jobName,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "terraform-tfvars",
										Path: "terraform.tfvars",
									},
									{
										Key:  "backend-tf",
										Path: "backend.tf",
									},
								},
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

func TestJob(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ws := &desiredTestWorkspaceForJob
	j := &desiredJobObject

	// Test getEnv function
	env := getEnv(ws)
	g.Expect(env).NotTo(gomega.BeEmpty())
	g.Expect(env).Should(gomega.Equal(desiredTestEnvVar))

	// Test StartJob function
	job := StartJob(jobName, jobNamespace, tfCommandTemplate, ws)
	g.Expect(job).Should(gomega.Equal(j))
}
