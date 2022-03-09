package controllers

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	replikav1alpha1 "prosimcorp.com/replika/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultSynchronizationTime = 15 * time.Second
	defaultTargetNamespace     = "default"
	namespaceRegularExpression = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

	// The Replika CR which created the resource
	resourceReplikaLabelPartOfKey   = "replika.prosimcorp.com/part-of"
	resourceReplikaLabelPartOfValue = ""

	// Who is managing the child resources
	resourceReplikaLabelCreatedKey   = "replika.prosimcorp.com/created-by"
	resourceReplikaLabelCreatedValue = "replika-controller"

	// Define the finalizers for handling deletion
	replikaFinalizer = "replika.prosimcorp.com/finalizer"
)

// GetNamespaces Returns the target namespaces of a Replika as a golang list
// The namespace of the replicated source is NEVER listed to avoid overwrites
func (r *ReplikaReconciler) GetNamespaces(ctx context.Context, replika *replikav1alpha1.Replika) (namespaces []string, err error) {

	// Loop and check the targets given by the user
	var expression *regexp.Regexp
	expression, err = regexp.Compile(namespaceRegularExpression)
	if err != nil {
		return namespaces, err
	}

	// List ALL namespaces without blacklisted ones
	if replika.Spec.Target.Namespaces.MatchAll {

		namespaceList := &corev1.NamespaceList{}
		err = r.List(ctx, namespaceList)
		if err != nil {
			return namespaces, err
		}

		// Convert Namespace Objects into Strings
	namespaceLoop:
		for _, v := range namespaceList.Items {
			ns := v.GetName()

			// Do NOT include the namespace of the replicated source to avoid possible overwrites
			if ns == replika.Spec.Source.Namespace {
				continue
			}

			// Exclude blacklisted namespaces
			for _, excludedNs := range replika.Spec.Target.Namespaces.ExcludeFrom {

				// Namespaces must be well formatted
				if !expression.Match([]byte(excludedNs)) {
					err = NewErrorf(namespaceFormatError, excludedNs)
					return namespaces, err
				}

				if excludedNs == ns {
					continue namespaceLoop
				}
			}
			namespaces = append(namespaces, ns)
		}

		return namespaces, err
	}

	// Empty list of targets, only 'default' included
	if len(replika.Spec.Target.Namespaces.ReplicateIn) == 0 {
		if replika.Spec.Source.Namespace != defaultTargetNamespace {
			namespaces = append(namespaces, defaultTargetNamespace)
			return namespaces, err
		}

		err = NewErrorf(sourceAndTargetSameNamespaceError, defaultTargetNamespace)
		return namespaces, err
	}

	for _, v := range replika.Spec.Target.Namespaces.ReplicateIn {
		if v == replika.Spec.Source.Namespace {
			err = NewErrorf(sourceAndTargetSameNamespaceError, v)
		}

		if !expression.Match([]byte(v)) {
			err = NewErrorf(namespaceFormatError, v)
			return namespaces, err
		}

		namespaces = append(namespaces, v)
	}

	return namespaces, err
}

// GetSynchronizationTime return the spec.synchronization.time as duration, or default time on failures
func (r *ReplikaReconciler) GetSynchronizationTime(replika *replikav1alpha1.Replika) (synchronizationTime time.Duration, err error) {
	synchronizationTime, err = time.ParseDuration(replika.Spec.Synchronization.Time)
	if err != nil {
		synchronizationTime = defaultSynchronizationTime
		err = NewErrorf(parseSyncTimeError, replika.Name)
		return synchronizationTime, err
	}

	return synchronizationTime, err
}

// GetSource return the source resource that will be replicated
func (r *ReplikaReconciler) GetSource(ctx context.Context, replika *replikav1alpha1.Replika) (source *unstructured.Unstructured, err error) {

	// Get the source manifest
	source = &unstructured.Unstructured{}
	source.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replika.Spec.Source.Group,
		Kind:    replika.Spec.Source.Kind,
		Version: replika.Spec.Source.Version,
	})

	err = r.Get(ctx, client.ObjectKey{
		Namespace: replika.Spec.Source.Namespace,
		Name:      replika.Spec.Source.Name,
	}, source)

	return source, err
}

// BuildTargets return a list with all the targets that will be created using the source
func (r *ReplikaReconciler) BuildTargets(ctx context.Context, replika *replikav1alpha1.Replika) (targets []unstructured.Unstructured, err error) {

	// Get the source from a replika
	var source *unstructured.Unstructured
	source, err = r.GetSource(ctx, replika)
	if err != nil {
		r.UpdateReplikaCondition(replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
			metav1.ConditionFalse,
			ConditionReasonSourceNotFound,
			ConditionReasonSourceNotFoundMessage,
		))
		return targets, err
	}

	// Get the namespaces to generate targets
	var namespaces []string
	namespaces, err = r.GetNamespaces(ctx, replika)
	if err != nil {
		r.UpdateReplikaCondition(replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
			metav1.ConditionFalse,
			ConditionReasonTargetNamespaceNotFound,
			ConditionReasonTargetNamespaceNotFoundMessage,
		))
		return targets, err
	}

	// Copy source object and generate a clean target object
	target := source.DeepCopy()
	unstructured.RemoveNestedField(target.Object, "metadata")
	unstructured.RemoveNestedField(target.Object, "status")
	target.SetName(source.GetName())
	target.SetAnnotations(source.GetAnnotations())

	labels := make(map[string]string)
	for k, v := range source.GetLabels() {
		labels[k] = v
	}
	labels[resourceReplikaLabelCreatedKey] = resourceReplikaLabelCreatedValue
	labels[resourceReplikaLabelPartOfKey] = replika.Name

	target.SetLabels(labels)

	// Add a new target to the list changing the namespace
	targets = []unstructured.Unstructured{}
	for _, ns := range namespaces {
		target.SetNamespace(ns)
		targets = append(targets, *target.DeepCopy())
	}

	return targets, err
}

// UpdateTarget Update a target, or create when not existent
func (r *ReplikaReconciler) UpdateTarget(ctx context.Context, target *unstructured.Unstructured) (err error) {

	// Look for the target in the target namespace
	tmpTarget := target.DeepCopy()
	err = r.Get(ctx, client.ObjectKey{
		Namespace: target.GetNamespace(),
		Name:      tmpTarget.GetName(),
	}, tmpTarget)

	// Create the resource when it is not found
	if err != nil {
		err = r.Create(ctx, target.DeepCopy())
		return err
	}

	// Update the object
	patch, err := target.MarshalJSON()
	err = r.Patch(ctx, target, client.RawPatch(types.MergePatchType, patch))

	return err
}

// UpdateTargets Synchronizes all the targets from a source declared on a Replika
func (r *ReplikaReconciler) UpdateTargets(ctx context.Context, replika *replikav1alpha1.Replika) (err error) {

	// Get a list of manifests for all the targets
	var targets []unstructured.Unstructured
	targets, err = r.BuildTargets(ctx, replika)
	if err != nil {
		return err
	}

	// Create the resource inside target namespaces
	// Needed to create a copy and change the namespace between loops
	for i := range targets {
		err = r.UpdateTarget(ctx, &targets[i])
		if err != nil {
			r.UpdateReplikaCondition(replika, r.NewReplikaCondition(ConditionTypeSourceSynced,
				metav1.ConditionFalse,
				ConditionReasonSourceReplicationFailed,
				ConditionReasonSourceReplicationFailedMessage,
			))
			return err
		}
	}

	return err
}

// DeleteTargets Delete all the targets previously created from a source declared on a Replika
func (r *ReplikaReconciler) DeleteTargets(ctx context.Context, replika *replikav1alpha1.Replika) (err error) {

	// Construct a target list object
	targets := &unstructured.UnstructuredList{}
	targets.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replika.Spec.Source.Group,
		Kind:    replika.Spec.Source.Kind,
		Version: replika.Spec.Source.Version,
	})

	// Look for the targets inside the cluster
	err = r.List(ctx, targets, client.MatchingLabels{resourceReplikaLabelPartOfKey: replika.Name})
	if err != nil {
		return err
	}

	// Delete the targets
	for i := range targets.Items {
		err = r.Delete(ctx, &targets.Items[i])
		if err != nil {
			return err
		}
	}

	return err
}
