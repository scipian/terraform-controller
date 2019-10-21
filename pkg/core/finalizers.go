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

package core

// AddFinalizer adds the finalizer to the given Object
func AddFinalizer(finalizerString string, obj Object) {
	finalizers := obj.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == finalizerString {
			// Object already contains the finalizer
			return
		}
	}

	//Object doesn't contain the finalizer, so add it
	finalizers = append(finalizers, finalizerString)
	obj.SetFinalizers(finalizers)
}

// RemoveFinalizer removes the finalizer from the given Object
func RemoveFinalizer(finalizerString string, obj Object) {
	finalizers := obj.GetFinalizers()

	// Filter existing finalizers removing any that match the finalizerString
	newFinalizers := []string{}
	for _, finalizer := range finalizers {
		if finalizer != finalizerString {
			newFinalizers = append(newFinalizers, finalizer)
		}
	}

	// Update the Object's finalizers
	obj.SetFinalizers(newFinalizers)
}

// HasFinalizer checks for the presence of the finalizer
func HasFinalizer(finalizerString string, obj Object) bool {
	finalizers := obj.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == finalizerString {
			// Object already contains the finalizer
			return true
		}
	}

	return false
}
