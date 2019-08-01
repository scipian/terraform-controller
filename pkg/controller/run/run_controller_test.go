// /*
// Copyright 2018 The Scipian Team.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package run

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
var runName = "fake-run-1"
var runKey = types.NamespacedName{Name: runName, Namespace: "default"}
var expectedRunRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: runName, Namespace: "default"}}
var expectedWorkspaceRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "fake", Namespace: "default"}}

const timeout = time.Second * 5

var ws = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "fake",
		Namespace: "default",
	},
	Spec: terraformv1alpha1.WorkspaceSpec{
		Image:      "quay.io/fake-image",
		Secret:     "fake-secret",
		WorkingDir: "/fake",
		Region:     "us-west-2",
		EnvVars:    map[string]string{"FOO": "foo"},
		TfVars:     map[string]string{"BAR": "bar"},
	},
}

var r = terraformv1alpha1.Run{
	ObjectMeta: metav1.ObjectMeta{
		Name:      runName,
		Namespace: "default",
	},
	Spec: terraformv1alpha1.RunSpec{
		WorkspaceName: "fake",
	},
}

var s = corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "scipian-aws-iam-creds",
		Namespace: "scipian",
	},
	StringData: map[string]string{"access-key": "test-key", "secret-key": "test-secret"},
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	workspace := &ws
	run := &r
	secret := &s

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the Workspace object for the Run to reference
	err = c.Create(context.TODO(), workspace)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), workspace)

	// Create Secret object for Run to reference
	err = c.Create(context.TODO(), secret)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), secret)

	// Create the Run object and expect Config map and job to be Created
	err = c.Create(context.TODO(), run)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), run)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRunRequest)))

	configMap := &corev1.ConfigMap{}
	job := &batchv1.Job{}

	g.Eventually(func() error { return c.Get(context.TODO(), runKey, configMap) }, timeout).
		Should(gomega.Succeed())
	g.Eventually(func() error { return c.Get(context.TODO(), runKey, job) }, timeout).
		Should(gomega.Succeed())

	// Delete the ConfigMap and Job
	g.Expect(c.Delete(context.TODO(), configMap)).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRunRequest)))
	g.Eventually(func() error { return c.Get(context.TODO(), runKey, configMap) }, timeout).
		Should(gomega.Succeed())

	g.Expect(c.Delete(context.TODO(), job)).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRunRequest)))
	g.Eventually(func() error { return c.Get(context.TODO(), runKey, job) }, timeout).
		Should(gomega.Succeed())

	// Manually delete ConfigMap and Job since GC isn't enabled in the test control plane
	g.Expect(c.Delete(context.TODO(), configMap)).To(gomega.Succeed())
	g.Expect(c.Delete(context.TODO(), job)).To(gomega.Succeed())

	// remove finalizers so Workspace can be deleted (defered earlier)
	workspace.ObjectMeta.Finalizers = nil
	workspace.Finalizers = nil

}
