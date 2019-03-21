/*
Copyright 2018 The Scipian Team.

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

package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/onsi/gomega"
	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var workspaceKey = types.NamespacedName{Name: "foo", Namespace: "default"}
var deleteWorkspaceKey = types.NamespacedName{Name: "foo-delete", Namespace: "default"}

const timeout = time.Second * 5

var ws = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "foo",
		Namespace: "default",
	},
	Spec: terraformv1alpha1.WorkspaceSpec{
		Image:      "quay.io/fake-image",
		Secret:     "fake-secret",
		WorkingDir: "/foo",
		EnvVars:    map[string]string{"FOO": "foo"},
		TfVars:     map[string]string{"BAR": "bar"},
		Backend: terraformv1alpha1.WorkspaceBackend{
			Type:          "test-type",
			Bucket:        "test-bucket",
			Key:           "test-key",
			Region:        "test-region",
			DynamoDBTable: "test-table",
		},
	},
}

var backOffLimit int32

var j = batchv1.Job{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Job",
		APIVersion: "batch/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "foo-job",
		Namespace: "default",
	},
	Spec: batchv1.JobSpec{
		BackoffLimit: &backOffLimit,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo-job",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:       "foo-job",
						Image:      "foo-job",
						Command:    []string{"/bin/ash"},
						Args:       []string{"-c", "foo-job"},
						WorkingDir: "/foo-job",
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		},
	},
	Status: batchv1.JobStatus{
		Succeeded: 0,
		Failed:    0,
	},
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	workspace := &ws

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	var createWorkspace = func(t *testing.T) {
		// Create the Workspace object and expect a ConfigMap and Job to be created
		err = c.Create(context.TODO(), workspace)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

		// Check that workspace cointains our finalizer
		err = c.Get(context.TODO(), workspaceKey, workspace)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(workspace.Finalizers[0]).NotTo(gomega.BeNil())
		g.Expect(workspace.Finalizers[0]).Should(gomega.Equal("workspace.finalizer.scipian.io"))

		// Verify Job and ConfigMap have been created
		job := &batchv1.Job{}
		cm := &corev1.ConfigMap{}
		g.Eventually(func() error { return c.Get(context.TODO(), workspaceKey, job) }, timeout).
			Should(gomega.Succeed())
		g.Eventually(func() error { return c.Get(context.TODO(), workspaceKey, cm) }, timeout).
			Should(gomega.Succeed())

		// Manually delete Deployment since GC isn't enabled in the test control plane
		g.Expect(c.Delete(context.TODO(), job)).To(gomega.Succeed())
		g.Expect(c.Delete(context.TODO(), cm)).To(gomega.Succeed())

	}

	var deleteWorkspace = func(t *testing.T) {
		// Trigger a Delete event on the Workspace
		err = c.Delete(context.TODO(), workspace)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		// Check that workspace still exists with finalizer
		err = c.Get(context.TODO(), workspaceKey, workspace)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(workspace.Finalizers[0]).NotTo(gomega.BeNil())
		g.Expect(workspace.Finalizers[0]).Should(gomega.Equal("workspace.finalizer.scipian.io"))

		// Verify Job and ConfigMap have been created
		job := &batchv1.Job{}
		cm := &corev1.ConfigMap{}
		g.Eventually(func() error { return c.Get(context.TODO(), deleteWorkspaceKey, job) }, timeout).
			Should(gomega.Succeed())
		g.Eventually(func() error { return c.Get(context.TODO(), deleteWorkspaceKey, cm) }, timeout).
			Should(gomega.Succeed())

		// Update job status to Succeeded
		var succeeded int32 = 1
		job.Status.Succeeded = succeeded
		err = c.Update(context.TODO(), job)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		// TODO(NL): Look into why updating the status to the job object here
		// isn't being reflected in the job status referenced by the workspace
		// controller (in the removeFinalizer func). You can log the status here
		// and get succeeded 1 (expected), and log in the removeFinalizer func
		// and get succeeded 0.
		// GitHub issue reference: https://github.com/scipian/terraform-controller/issues/10#issue-436410269

		// Manually delete Deployment since GC isn't enabled in the test control plane
		g.Expect(c.Delete(context.TODO(), job)).To(gomega.Succeed())
		g.Expect(c.Delete(context.TODO(), cm)).To(gomega.Succeed())

		// final cleanup to make sure finalizers were removed and workspace is deleted
		workspace.ObjectMeta.Finalizers = nil
		c.Update(context.TODO(), workspace)
		c.Delete(context.TODO(), workspace)
	}

	createWorkspace(t)
	deleteWorkspace(t)
}
