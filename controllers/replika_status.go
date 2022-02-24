package controllers

import (
	"context"
	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// https://github.com/external-secrets/external-secrets/blob/80545f4f183795ef193747fc959558c761b51c99/apis/externalsecrets/v1alpha1/externalsecret_types.go#L168
const (
	// ConditionTypeSourceSynced indicates that the source was synchronizated or not
	ConditionTypeSourceSynced = "SourceSynced"

	ConditionReasonSourceNotFound             = "SourceNotFound"
	ConditionReasonTargetNamespaceNotFound    = "TargetNamespaceNotFound"
	ConditionReasonSourceNamespaceNotFound    = "SourceNamespaceNotFound"
	ConditionReasonSourceReplicationFailed    = "SourceReplicationFailed"
	ConditionReasonSourceReplicationInProcess = "SourceReplicationInProcess"
	ConditionReasonSourceSynced               = "SourceSynced"
)

// NewReplikaCondition a set of default options for creating a Replika Condition.
func (r *ReplikaReconciler) NewReplikaCondition(condType string, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetReplikaCondition returns the condition with the provided type.
func (r *ReplikaReconciler) GetReplikaCondition(replika *replikav1alpha1.Replika, condType string) *metav1.Condition {

	for i, v := range replika.Status.Conditions {
		if v.Type == condType {
			return &replika.Status.Conditions[i]
		}
	}
	return nil
}

// UpdateReplikaCondition update or create a condition inside the status
func (r *ReplikaReconciler) UpdateReplikaCondition(ctx context.Context, replika *replikav1alpha1.Replika, condition *metav1.Condition) (err error) {

	// Get the condition
	currentCondition := r.GetReplikaCondition(replika, condition.Type)

	if currentCondition == nil {
		// Create the condition when not existent
		replika.Status.Conditions = append(replika.Status.Conditions, *condition)
	} else {
		// Update the condition when existent.
		currentCondition.Status = condition.Status
		currentCondition.Reason = condition.Reason
		currentCondition.Message = condition.Message
		currentCondition.LastTransitionTime = metav1.Now()
	}

	return err
}

// UpdateAndLogReplikaCondition update a condition on a replika status and throw the status message on logs
func (r *ReplikaReconciler) UpdateAndLogReplikaCondition(ctx context.Context, replika *replikav1alpha1.Replika, condition *metav1.Condition) (err error) {

	err = r.UpdateReplikaCondition(ctx, replika, condition)
	if err != nil {
		log.Log.Error(err, "Impossible to update the condition on replika "+replika.Name)
		return err
	}

	log.Log.Info(condition.Message)
	return err
}
