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

package run

import (
	"context"
	"log"
	"reflect"

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
	tfPlan    = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace select %s && terraform plan -input=false -out=plan.bin && terraform apply -input=false plan.bin"
	tfDestroy = "cp /opt/meta/* %s && terraform init -force-copy && terraform workspace select %s && terraform destroy -auto-approve"
)

var (
	falseVal = false
	trueVal  = true
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Run Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this terraform.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRun{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("run-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Run
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Run{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &terraformv1alpha1.Run{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &terraformv1alpha1.Run{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileRun{}

// ReconcileRun reconciles a Run object
type ReconcileRun struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Run object and makes changes based on the state read
// and what is in the Run.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=runs,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileRun) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var (
		terraformCmd string
		err          error
	)

	run := &terraformv1alpha1.Run{}
	workspace := &terraformv1alpha1.Workspace{}

	err = r.Get(context.TODO(), request.NamespacedName, run)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	err = r.Get(context.TODO(), types.NamespacedName{Name: run.Spec.WorkspaceName, Namespace: run.Namespace}, workspace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if run.Spec.DestroyResource == true {
		terraformCmd = tfDestroy
	} else {
		terraformCmd = tfPlan
	}

	configMap := terraform.CreateConfigMap(run.Name, run.Namespace, workspace)
	runJob := terraform.StartJob(run.Name, run.Namespace, terraformCmd, workspace)

	if err := r.setControllerReference(run, configMap); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.setControllerReference(run, runJob); err != nil {
		return reconcile.Result{}, err
	}

	foundConfigMap := &corev1.ConfigMap{}
	foundRunJob := &batchv1.Job{}

	// Create ConfigMap and Job
	if err := r.createObject(configMap.Name, configMap.Namespace, configMap, foundConfigMap, "ConfigMap"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.createObject(runJob.Name, runJob.Namespace, runJob, foundRunJob, "Job"); err != nil {
		return reconcile.Result{}, err
	}

	// Update ConfigMap and Job
	if err := r.updateObject(configMap.Name, configMap.Namespace, configMap.Data, foundConfigMap.Data, foundConfigMap, "ConfigMap"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateObject(runJob.Name, runJob.Namespace, runJob.Spec, foundRunJob.Spec, foundRunJob, "Job"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileRun) setControllerReference(owner, reference v1.Object) error {
	if err := controllerutil.SetControllerReference(owner, reference, r.scheme); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileRun) createObject(name string, namespace string, createObject runtime.Object, foundObject runtime.Object, objectKind string) error {
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

func (r *ReconcileRun) updateObject(name string, namespace string, actualObjectField interface{}, foundObjectField interface{}, objectToUpdate runtime.Object, objectKind string) error {
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
