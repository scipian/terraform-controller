package terraform

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Job", func() {

	var (
		jobName           = "test-job"
		jobNamespace      = "test-namespace"
		key               = types.NamespacedName{Namespace: jobNamespace, Name: jobName}
		secretName        = "test-secret"
		image             = "test-image"
		workDir           = "test-working-dir"
		tfCommandTemplate = "foo %s %s"
		desiredTfCommand  = fmt.Sprintf(tfCommandTemplate, workDir, jobName)
	)

	// Set up desired corev1.EnvVar object
	desiredTestEnvVar := []corev1.EnvVar{
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
	desiredTestWorkspaceForJob := terraformv1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: terraformv1.WorkspaceSpec{
			EnvVars:    map[string]string{"FOO": "foo"},
			Secret:     secretName,
			Image:      image,
			WorkingDir: workDir,
		},
	}

	// Set up desired Job object
	desiredJobObject := batchv1.Job{
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

	Context("Get ENVs", func() {
		It("Should create ENV object", func() {
			ws := &desiredTestWorkspaceForJob
			env := getEnv(ws)
			Expect(env).NotTo(BeEmpty())
			Expect(env).Should(Equal(desiredTestEnvVar))
		})
	})
	Context("Create job", func() {
		It("Should create job object", func() {
			ws := &desiredTestWorkspaceForJob
			j := &desiredJobObject
			job := CreateJob(key, tfCommandTemplate, ws)
			Expect(job).Should(Equal(j))
		})
	})
})
