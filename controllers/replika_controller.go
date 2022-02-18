/*
Copyright 2022.

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"
)

const (
	// The Replika CR which created the resource
	resourceReplikaLabelPartKey   = "replika.prosimcorp.com/part-of"
	resourceReplikaLabelPartValue = ""

	// Who is managing the resources
	resourceReplikaLabelCreatedKey   = "replika.prosimcorp.com/created-by"
	resourceReplikaLabelCreatedValue = "replika-controller"
)

// ReplikaReconciler reconciles a Replika object
type ReplikaReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ReplikaInstance *replikav1alpha1.Replika
	QuitGoroutine   bool
}

// GetNamespaces Returns the namespaces of a Replika as a golang list
func (r *ReplikaReconciler) GetNamespaces(str string) ([]string, error) {
	str = strings.TrimSpace(str)

	if str == "" {
		return []string{"default"}, nil
	}

	if str == "*" {
		namespace := &corev1.NamespaceList{}
		err := r.List(context.Background(), namespace)
		if err != nil {
			return nil, err
		}

		list := make([]string, namespace.Size())
		for _, v := range namespace.Items {
			list = append(list, v.GetNamespace())
		}
		return list, nil
	}

	list := strings.Split(str, ",")

	return list, nil
}

// UpdateTarget Update a target. The up-to-date spec comes from outside
func (r *ReplikaReconciler) UpdateTarget(ctx context.Context, target *unstructured.Unstructured) (err error) {

	// Look for the target in the target namespace
	tmpTarget := target.DeepCopy()
	log.Log.Info("looking for target resource in namespace '" + target.GetNamespace() + "'...")
	err = r.Get(ctx, client.ObjectKey{
		Namespace: target.GetNamespace(),
		Name:      tmpTarget.GetName(),
	}, tmpTarget)

	// Create the resource when it is not found
	if err != nil {
		log.Log.Info("creating target resource in namespace '" + target.GetNamespace() + "'...")
		err = r.Create(ctx, target.DeepCopy())
		if err != nil {
			log.Log.Error(err, "can not create the resource inside namespace '"+target.GetNamespace()+"'")
			return err
		}
		log.Log.Info("target resource created successfully in namespace '" + target.GetNamespace() + "'")
		return err
	}

	log.Log.Info("target resource found successfully in namespace '" + target.GetNamespace() + "'")

	// Objects in Kubernetes are uniquely identified
	// Set target UID to the UID received from de created resource
	target.SetUID(tmpTarget.GetUID())

	// Update the object
	log.Log.Info("updating target resource in namespace '" + target.GetNamespace() + "'...")
	err = r.Update(ctx, target.DeepCopy())
	if err != nil {
		log.Log.Error(err, "can not update the resource inside namespace '"+target.GetNamespace()+"'")
		return err
	}
	log.Log.Info("target resource update successfully in namespace '" + target.GetNamespace() + "'")

	return err
}

// DeleteTargets delete all the targets that matches labels of the Replika CR which created them
func (r *ReplikaReconciler) DeleteTargets(ctx context.Context) (err error) {

	// Get a Go list of namespaces from the declared string
	var namespaces []string
	namespaces, err = r.GetNamespaces(r.ReplikaInstance.Spec.Target.Namespaces)
	if err != nil {
		log.Log.Error(err, "can not get the namespaces to delete resources")
	}

	// Construct a target object
	target := &unstructured.Unstructured{}
	target.SetName(r.ReplikaInstance.Spec.Source.Name)
	target.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   r.ReplikaInstance.Spec.Source.Group,
		Kind:    r.ReplikaInstance.Spec.Source.Kind,
		Version: r.ReplikaInstance.Spec.Source.Version,
	})

	// Delete the resource inside target namespaces
	for _, ns := range namespaces {
		target.SetNamespace(ns)
		err = r.Delete(ctx, target)
		if err != nil {
			log.Log.Error(err, "can not delete the resource inside namespace '"+ns+"'")
		}
	}

	return err
}

// AsyncReconcile Syncs the manifests asynchronously
func (r *ReplikaReconciler) AsyncReconcile(ctx context.Context) {
	var err error

	// Calculate the duration of the synchronization
	var durationTime time.Duration
	durationTime, err = time.ParseDuration(r.ReplikaInstance.Spec.Synchronization.Time)
	if err != nil {
		log.Log.Error(err, "can not parse the synchronization time.")
	}

	// Get a Go list of namespaces from the declared string
	var namespaces []string
	namespaces, err = r.GetNamespaces(r.ReplikaInstance.Spec.Target.Namespaces)
	if err != nil {
		log.Log.Error(err, "can not get the namespaces")
	}

	for {
		// Get the source manifest
		source := unstructured.Unstructured{}
		source.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   r.ReplikaInstance.Spec.Source.Group,
			Kind:    r.ReplikaInstance.Spec.Source.Kind,
			Version: r.ReplikaInstance.Spec.Source.Version,
		})

		log.Log.Info("looking for source resource...")
		err = r.Get(ctx, client.ObjectKey{
			Namespace: r.ReplikaInstance.Spec.Source.Namespace,
			Name:      r.ReplikaInstance.Spec.Source.Name,
		}, &source)
		if err != nil {
			log.Log.Error(err, "can not find the resource inside namespace '"+r.ReplikaInstance.Spec.Source.Namespace+"'")
			time.Sleep(durationTime)
		}
		log.Log.Info("source resource found successfully")

		// Copy source object and clean trash from the manifest
		target := source.DeepCopy()
		unstructured.RemoveNestedField(target.Object, "metadata")
		unstructured.RemoveNestedField(target.Object, "status")

		// Set the necessary metadata
		target.SetName(source.GetName())
		target.SetAnnotations(source.GetAnnotations())

		labels := make(map[string]string)
		for k, v := range source.GetLabels() {
			labels[k] = v
		}
		labels[resourceReplikaLabelCreatedKey] = resourceReplikaLabelCreatedValue
		labels[resourceReplikaLabelPartKey] = r.ReplikaInstance.Name

		target.SetLabels(labels)

		// Create the resource inside target namespaces
		// Needed to create a copy and change the namespace between loops
		for _, ns := range namespaces {
			target.SetNamespace(ns)

			//
			err = r.UpdateTarget(ctx, target)
			if err != nil {
				log.Log.Error(err, "can not update the resource inside namespace '"+ns+"'")
			}
		}

		log.Log.Info("waiting for next review in " + durationTime.String())
		// Sleep until next reconcile time
		time.Sleep(durationTime)

		// Build a killer for the goroutine
		if r.QuitGoroutine {
			return
		}
	}
}

//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReplikaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Create the asynchronous reconciler by default
	r.QuitGoroutine = false

	// Get the content of the Replika
	// Kill the synchronization and resources
	replikaManifest := &replikav1alpha1.Replika{}
	err := r.Get(ctx, req.NamespacedName, replikaManifest)
	if err != nil {
		r.QuitGoroutine = true
		if !strings.Contains(err.Error(), "not found") {
			log.Log.Error(err, "")
		}
		err = r.DeleteTargets(ctx)
		if err != nil {
			log.Log.Error(err, "can not delete the replicas")
		}
		return ctrl.Result{}, nil
	}

	r.ReplikaInstance = replikaManifest.DeepCopy()

	// Launch an asynchronous reconciler
	go r.AsyncReconcile(ctx)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplikaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replikav1alpha1.Replika{}).
		Complete(r)
}
