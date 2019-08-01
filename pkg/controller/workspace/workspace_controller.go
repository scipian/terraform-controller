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
	"fmt"
	"log"
	"reflect"
	"time"

	terraformv1alpha1 "github.com/scipian/terraform-controller/pkg/apis/terraform/v1alpha1"
	"github.com/scipian/terraform-controller/pkg/terraform"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	tfWorkspaceNew       = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace new %s"
	tfWorkspaceDelete    = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace delete -force %s"
	scipianIAMSecretName = "scipian-aws-iam-creds"
	scipianNamespace     = "scipian"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Workspace Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this terraform.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWorkspace{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("workspace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Workspace
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Workspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &terraformv1alpha1.Workspace{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &terraformv1alpha1.Workspace{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileWorkspace{}

// ReconcileWorkspace reconciles a Workspace object
type ReconcileWorkspace struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Workspace object and makes changes based on the state read
// and what is in the Workspace.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.
// Automatically generate RBAC rules to allow the Controller to read and write resources
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileWorkspace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	workspaceFinalizerName := "workspace.finalizer.scipian.io"
	workspace := &terraformv1alpha1.Workspace{}

	err := r.Get(context.TODO(), request.NamespacedName, workspace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if workspace.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object.
		if !containsString(workspace.ObjectMeta.Finalizers, workspaceFinalizerName) {

			workspace.ObjectMeta.Finalizers = append(workspace.ObjectMeta.Finalizers, workspaceFinalizerName)
			if err := r.Update(context.Background(), workspace); err != nil {
				return reconcile.Result{}, err
			}
		}

		if err := r.startJob(workspace.Name, tfWorkspaceNew, workspace); err != nil {
			return reconcile.Result{}, err
		}

	} else {
		// The object is being deleted
		log.Printf("Deleting the external dependencies")
		jobName := fmt.Sprintf("%s-delete", workspace.Name)

		if containsString(workspace.ObjectMeta.Finalizers, workspaceFinalizerName) {
			if err := r.startJob(jobName, tfWorkspaceDelete, workspace); err != nil {
				// TODO(NL): There is an error here around label and label selectors
				// not being present or not matching.
				// GitHub issue reference: https://github.com/scipian/terraform-controller/issues/8#issue-434520188

				// log.Print(err)
				// return reconcile.Result{}, err
			}

			if err := r.removeFinalizer(jobName, workspaceFinalizerName, workspace); err != nil {
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, nil
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileWorkspace) startJob(jobName string, terraformCmd string, workspace *terraformv1alpha1.Workspace) error {
	foundWorkspaceJob := &batchv1.Job{}
	foundConfigMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}

	scipianIAMSecret, err := r.getSecret(scipianIAMSecretName, scipianNamespace, secret)
	if err != nil {
		return err
	}

	accessKey := scipianIAMSecret.StringData["access-key"]
	secretKey := scipianIAMSecret.StringData["secret-key"]

	configMap := terraform.CreateConfigMap(jobName, workspace.Namespace, accessKey, secretKey, workspace)
	workspaceJob := terraform.StartJob(jobName, workspace.Namespace, terraformCmd, workspace)

	if err := r.setControllerReference(workspace, configMap); err != nil {
		return err
	}

	if err := r.setControllerReference(workspace, workspaceJob); err != nil {
		return err
	}

	// Create ConfigMap and Job
	if err := r.createObject(configMap.Name, configMap.Namespace, configMap, foundConfigMap, "ConfigMap"); err != nil {
		return err
	}

	if err := r.createObject(workspaceJob.Name, workspaceJob.Namespace, workspaceJob, foundWorkspaceJob, "Job"); err != nil {
		return err
	}

	// Update ConfigMap and Job
	if err := r.updateObject(configMap.Name, configMap.Namespace, configMap.Data, foundConfigMap.Data, foundConfigMap, "ConfigMap"); err != nil {
		return err
	}

	if err := r.updateObject(workspaceJob.Name, workspaceJob.Namespace, workspaceJob.Spec, foundWorkspaceJob.Spec, foundWorkspaceJob, "Job"); err != nil {
		return err
	}

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *ReconcileWorkspace) removeFinalizer(jobName string, finalizerName string, workspace *terraformv1alpha1.Workspace) error {
	var succeededJobs int32 = 1
	var failedJobs int32 = 1
	foundJob := &batchv1.Job{}

	for {

		if err := r.Get(context.TODO(), types.NamespacedName{Name: jobName, Namespace: workspace.Namespace}, foundJob); err != nil {
			return err
		}

		if foundJob.Status.Succeeded == succeededJobs {
			log.Printf("Deleting finalizer: %s\n", finalizerName)
			workspace.ObjectMeta.Finalizers = removeString(workspace.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(context.Background(), workspace); err != nil {
				// TODO(NL): There is an error here about the UID not being present.
				// GitHub issue reference: https://github.com/scipian/terraform-controller/issues/7#issue-434519005
				return err
			}
			return nil
		}

		if foundJob.Status.Failed == failedJobs {
			return fmt.Errorf("job %s failed", foundJob.Name)
		}

		log.Printf("Waiting for %s workspace to destory\n", workspace.Name)
		time.Sleep(5 * time.Second)

		continue
	}
}

func (r *ReconcileWorkspace) setControllerReference(owner, reference v1.Object) error {
	if err := controllerutil.SetControllerReference(owner, reference, r.scheme); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileWorkspace) createObject(name string, namespace string, createObject runtime.Object, foundObject runtime.Object, objectKind string) error {
	err := r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundObject)
	if err != nil && errors.IsNotFound(err) {
		log.Printf("Creating %s %s/%s\n", objectKind, namespace, name)
		err = r.Create(context.TODO(), createObject)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileWorkspace) updateObject(name string, namespace string, actualObjectField interface{}, foundObjectField interface{}, objectToUpdate runtime.Object, objectKind string) error {
	if !reflect.DeepEqual(actualObjectField, foundObjectField) {
		foundObjectField = actualObjectField
		log.Printf("Updating %s %s/%s\n", objectKind, namespace, name)
		err := r.Update(context.TODO(), objectToUpdate)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileWorkspace) getSecret(name string, namespace string, secretObject *corev1.Secret) (corev1.Secret, error) {
	err := r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, secretObject)
	if err != nil {
		return *secretObject, err
	}
	return *secretObject, nil
}
