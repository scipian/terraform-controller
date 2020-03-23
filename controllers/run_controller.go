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
	"fmt"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	"github.com/scipian/terraform-controller/pkg/core"
	"github.com/scipian/terraform-controller/pkg/terraform"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// RunReconciler reconciles a Run object
type RunReconciler struct {
	Reconciler
}

// +kubebuilder:rbac:groups=terraform.scipian.io,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=terraform.scipian.io,resources=runs/list,verbs=get;update;patch

// Reconcile is the reconciler function for Run Custom Resources
func (r *RunReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var terraformCmd string
	destroyTrue := true
	run := &terraformv1.Run{}
	workspace := &terraformv1.Workspace{}

	ctx := context.Background()
	log := r.Log.WithValues("run", req.NamespacedName)

	if err := r.Get(ctx, req.NamespacedName, run); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to GET Run")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}

	if err := r.Get(ctx, types.NamespacedName{Name: run.Spec.WorkspaceName, Namespace: run.Namespace}, workspace); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to GET Workspace")
		}
		return ctrl.Result{}, ignoreNotFound(err)
	}

	if run.Spec.DestroyResource == destroyTrue {
		terraformCmd = core.TFDestroy
	} else {
		terraformCmd = core.TFPlan
	}

	if err := r.startJob(run, terraformCmd, workspace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.retrieveState(run, workspace); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager initializes the Run controller with the manager
func (r *RunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&terraformv1.Run{}).
		Complete(r)
}

func (r *RunReconciler) startJob(run *terraformv1.Run, terraformCmd string, workspace *terraformv1.Workspace) error {
	foundRunJob := &batchv1.Job{}
	foundConfigMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	runKey := types.NamespacedName{Namespace: run.Namespace, Name: run.Name}

	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	configMap := terraform.CreateConfigMap(runKey, iamAccessKey, iamSecretKey, workspace)
	runJob := terraform.CreateJob(runKey, terraformCmd, workspace)

	// Set Run as owner of configmap and job object
	if err := r.SetControllerReference(run, configMap); err != nil {
		return err
	}

	if err := r.SetControllerReference(run, runJob); err != nil {
		return err
	}

	// Create ConfigMap and Job
	if err := r.CreateObject(runKey, configMap, foundConfigMap); err != nil {
		return err
	}

	if err := r.CreateObject(runKey, runJob, foundRunJob); err != nil {
		return err
	}

	return nil
}

func (r *RunReconciler) retrieveState(run *terraformv1.Run, workspace *terraformv1.Workspace) error {
	var succeededJobs int32 = 1
	var failedJobs int32 = 1
	foundRunJob := &batchv1.Job{}
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: core.ScipianNamespace, Name: core.ScipianIAMSecretName}
	if err := r.GetSecret(secretKey, secret); err != nil {
		return err
	}
	iamAccessKey := string(secret.Data[core.AccessKey])
	iamSecretKey := string(secret.Data[core.SecretKey])

	for {

		if err := r.Get(context.TODO(), types.NamespacedName{Name: run.Name, Namespace: run.Namespace}, foundRunJob); err != nil {
			return err
		}

		if foundRunJob.Status.Succeeded == succeededJobs {
			log.Printf("Retrieving tfstate")
			state, err := core.RetrieveState(workspace, iamAccessKey, iamSecretKey)
			if err != nil {
				return fmt.Errorf("Error retrieving tfstate - %s", err)
			}
			workspace.Spec.TfState = state
			if err := r.Update(context.Background(), workspace); err != nil {
				return ignoreNotFound(err)
			}
			return nil
		}

		if foundRunJob.Status.Failed == failedJobs {
			return fmt.Errorf("job %s failed", foundRunJob.Name)
		}

		log.Printf("Waiting for %s/%s to complete successfully before syncing terraform state\n", foundRunJob.Namespace, foundRunJob.Name)
		time.Sleep(5 * time.Second)

		continue
	}
}
