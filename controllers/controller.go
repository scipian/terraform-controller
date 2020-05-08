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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciler reconciles a Kubernetes object
type Reconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// GetSecret retrieves a Kubernetes secret and unmarshalls the secret into a corev1.Secret struct
func (r *Reconciler) GetSecret(key types.NamespacedName, secretObject *corev1.Secret) error {
	err := r.Get(context.TODO(), key, secretObject)
	if err != nil {
		return err
	}
	return nil
}

// CreateObject creates a Kubernetes object based on given parameters
func (r *Reconciler) CreateObject(key types.NamespacedName, createObject runtime.Object, foundObject runtime.Object) error {
	obj := createObject.GetObjectKind().GroupVersionKind()
	kind := obj.Kind
	log := r.Log.WithValues(kind, key.Name)
	err := r.Get(context.TODO(), key, foundObject)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating", "Kind:", kind, "Namespace:", key.Namespace, "Name", key.Name)
		err = r.Create(context.TODO(), createObject)
		if err != nil {
			return err
		}
	} else if err != nil {
		return ignoreNotFound(err)
	}
	log.Info("Skipping Create", "Kind:", kind, "Namespace:", key.Namespace, "Name:", key.Name)
	return nil
}

// SetControllerReference sets an object to be owned by another object for garbage collecting
func (r *Reconciler) SetControllerReference(owner, reference v1.Object) error {
	if err := controllerutil.SetControllerReference(owner, reference, r.Scheme); err != nil {
		return err
	}
	return nil
}

func ignoreNotFound(err error) error {
	return client.IgnoreNotFound(err)
}
