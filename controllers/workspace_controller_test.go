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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	"github.com/scipian/terraform-controller/pkg/core"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("WorkspaceController", func() {
	const timeout = time.Second * 20
	const interval = time.Second * 1

	var workspaceKey = types.NamespacedName{Namespace: "default", Name: "foo"}
	var ctx = context.TODO()

	BeforeEach(func() {
		var err error

		s := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scipian-aws-iam-creds",
				Namespace: "scipian",
			},
			StringData: map[string]string{"access-key": "test-key", "secret-key": "test-secret"},
		}

		ws := terraformv1.Workspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: terraformv1.WorkspaceSpec{
				Image:      "quay.io/fake-image",
				Secret:     "fake-secret",
				WorkingDir: "/foo",
				Region:     "us-west-2",
				EnvVars:    map[string]string{"FOO": "foo"},
				TfVars:     map[string]string{"BAR": "bar"},
			},
		}

		secret := &s
		workspace := &ws

		By("Creating a Secret Object")
		err = k8sClient.Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to create test secret")

		By("Creating a Workspace Object")
		err = k8sClient.Create(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to create Foo Workspace")
	})

	AfterEach(func() {
		var err error

		secretKey := types.NamespacedName{
			Namespace: "scipian",
			Name:      "scipian-aws-iam-creds",
		}

		// Delete secret
		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, secretKey, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to GET secret")

		err = k8sClient.Delete(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE secret")

		// Remove finalizer from workspace and delete workspace
		workspace := &terraformv1.Workspace{}
		err = k8sClient.Get(ctx, workspaceKey, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to GET workspace")

		core.RemoveFinalizer(core.WorkspaceFinalizerName, workspace)

		err = k8sClient.Update(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to UPDATE workspace")

		err = k8sClient.Delete(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE workspace")
	})

	Context("Verify resources WorkspaceController creates", func() {
		It("Check if ConfigMap was created", func() {
			configMap := &corev1.ConfigMap{}

			Eventually(func() error {
				return k8sClient.Get(ctx, workspaceKey, configMap)
			}, timeout, interval).Should(Succeed())
		})

		It("Check if Job was created", func() {
			job := &batchv1.Job{}

			Eventually(func() error {
				return k8sClient.Get(ctx, workspaceKey, job)
			}, timeout, interval).Should(Succeed())
		})

		It("Verifying Workspace has a finalizer", func() {
			workspace := &terraformv1.Workspace{}

			Eventually(func() error {
				return k8sClient.Get(ctx, workspaceKey, workspace)
			}, timeout, interval).Should(Succeed())

			Eventually(func() []string {
				k8sClient.Get(ctx, workspaceKey, workspace)
				return workspace.GetFinalizers()
			}, timeout, interval).ShouldNot(BeNil())

			Eventually(func() bool {
				k8sClient.Get(ctx, workspaceKey, workspace)
				finalizers := workspace.GetFinalizers()
				return finalizers[0] == core.WorkspaceFinalizerName
			}, timeout, interval).Should(BeTrue())
		})
	})
})
