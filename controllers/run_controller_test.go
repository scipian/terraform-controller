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

var _ = Describe("RunController", func() {
	const timeout = time.Second * 20
	const interval = time.Second * 1

	var runKey = types.NamespacedName{Namespace: "default", Name: "foo"}
	var ctx = context.TODO()

	BeforeEach(func() {
		var err error
		var destroyTrue bool

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

		r := terraformv1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: terraformv1.RunSpec{
				WorkspaceName:   "foo",
				DestroyResource: destroyTrue,
			},
		}

		secret := &s
		workspace := &ws
		run := &r

		By("Creating a Secret Object")
		err = k8sClient.Create(ctx, secret)
		Expect(err).NotTo(HaveOccurred(), "failed to create test secret")

		By("Creating a Workspace Object")
		err = k8sClient.Create(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to create Foo Workspace")

		By("Creating a Run Object")
		err = k8sClient.Create(ctx, run)
		Expect(err).NotTo(HaveOccurred(), "failed to create Foo Run")
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

		// Delete run
		run := &terraformv1.Run{}
		err = k8sClient.Get(ctx, runKey, run)
		Expect(err).NotTo(HaveOccurred(), "failed to GET run")

		err = k8sClient.Delete(ctx, run)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE run")

		// Remove finalizer from workspace and delete workspace
		workspace := &terraformv1.Workspace{}
		err = k8sClient.Get(ctx, runKey, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to GET workspace")

		core.RemoveFinalizer(core.WorkspaceFinalizerName, workspace)

		err = k8sClient.Update(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to UPDATE workspace")

		err = k8sClient.Delete(ctx, workspace)
		Expect(err).NotTo(HaveOccurred(), "failed to DELETE workspace")
	})

	Context("Verify resources RunController creates", func() {
		It("Check if ConfigMap was created", func() {
			configMap := &corev1.ConfigMap{}

			Eventually(func() error {
				return k8sClient.Get(ctx, runKey, configMap)
			}, timeout, interval).Should(Succeed())
		})

		It("Check if Job was created", func() {
			job := &batchv1.Job{}

			Eventually(func() error {
				return k8sClient.Get(ctx, runKey, job)
			}, timeout, interval).Should(Succeed())
		})
	})
})
