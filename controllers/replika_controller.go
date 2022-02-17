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
	log2 "log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"

	runt "runtime"
)

// ReplikaReconciler reconciles a Replika object
type ReplikaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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

// AsyncReconcile Syncs the manifests asynchronously
func (r *ReplikaReconciler) AsyncReconcile(replika *replikav1alpha1.Replika) {
	var err error

	// Calculate the duration of the synchronization
	var durationTime time.Duration
	durationTime, err = time.ParseDuration(replika.Spec.Synchronization.Time)
	if err != nil {
		log.Log.Error(err, "can not parse the synchronization time.")
	}

	// Get a Go list of namespaces from the declared string
	var namespaces []string
	namespaces, err = r.GetNamespaces(replika.Spec.Target.Namespaces)
	if err != nil {
		log.Log.Error(err, "can not get the namespaces")
	}

	for {
		// Get the source manifest
		source := unstructured.Unstructured{}
		source.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   replika.Spec.Source.Group,
			Kind:    replika.Spec.Source.Kind,
			Version: replika.Spec.Source.Version,
		})

		log.Log.Info("looking for source resource...")
		err = r.Get(context.Background(), client.ObjectKey{
			Namespace: replika.Spec.Source.Namespace,
			Name:      replika.Spec.Source.Name,
		}, &source)
		if err != nil {
			log.Log.Error(err, "can not find the resource inside namespace '"+replika.Spec.Source.Namespace+"'")
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
		target.SetLabels(source.GetLabels())

		// Create the resource inside target namespaces
		// Needed to create a copy and change the namespace between loops
		for _, v := range namespaces {
			target.SetNamespace(v)

			// Look for the target in the target namespace
			tmpTarget := target.DeepCopy()
			log.Log.Info("looking for target resource in namespace '" + v + "'...")
			err = r.Get(context.Background(), client.ObjectKey{
				Namespace: v,
				Name:      tmpTarget.GetName(),
			}, tmpTarget)
			// Object not found, so create it
			if err != nil {
				log.Log.Info("creating target resource in namespace '" + v + "'...")
				err = r.Create(context.Background(), target.DeepCopy())
				if err != nil {
					log.Log.Error(err, "can not create the resource inside namespace '"+v+"'")
				}
				log.Log.Info("target resource created successfully in namespace '" + v + "'")
				continue
			}

			log.Log.Info("target resource found successfully in namespace '" + v + "'")

			// Set target UID to the UID received from de created resource
			target.SetUID(tmpTarget.GetUID())

			log.Log.Info("updating target resource in namespace '" + v + "'...")
			// Update the object
			err = r.Update(context.Background(), target.DeepCopy())
			if err != nil {
				log.Log.Error(err, "can not update the resource inside namespace '"+v+"'")
			}
			log.Log.Info("target resource update successfully in namespace '" + v + "'")

		}

		log.Log.Info("waiting for next review in " + durationTime.String())
		// Sleep until next reconcile time
		time.Sleep(durationTime)
	}
}

//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replika.prosimcorp.com,resources=replikas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Replika object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReplikaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	replikaManifest := &replikav1alpha1.Replika{}
	err := r.Get(ctx, req.NamespacedName, replikaManifest)

	if err != nil {
		// Do something
	}

	//ch := make(chan int)

	go r.AsyncReconcile(replikaManifest)

	log2.Print("goroutines: ", runt.NumGoroutine())

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplikaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replikav1alpha1.Replika{}).
		Complete(r)
}
