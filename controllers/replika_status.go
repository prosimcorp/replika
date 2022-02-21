package controllers

import (
	"context"
	"reflect"

	replikav1alpha1 "github.com/prosimcorp/replika/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// https://github.com/external-secrets/external-secrets/blob/80545f4f183795ef193747fc959558c761b51c99/apis/externalsecrets/v1alpha1/externalsecret_types.go#L168
const (
	ReplikaDeleted = "Deleted"
	ReplikaReady = "Ready"

	// Example -----------

	// ConditionReasonSecretSynced indicates that the secrets was synced.
	ConditionReasonSecretSynced = "SecretSynced"
	// ConditionReasonSecretSyncedError indicates that there was an error syncing the secret.
	ConditionReasonSecretSyncedError = "SecretSyncedError"
	// ConditionReasonSecretDeleted indicates that the secret has been deleted.
	ConditionReasonSecretDeleted = "SecretDeleted"

	ReasonInvalidStoreRef      = "InvalidStoreRef"
	ReasonProviderClientConfig = "InvalidProviderClientConfig"
	ReasonUpdateFailed         = "UpdateFailed"
	ReasonUpdated              = "Updated"
)

//
func (r *ReplikaReconciler) test(ctx context.Context, replika *replikav1alpha1.Replika) {

	condition := metav1.Condition{
		Type:               "SyncReplica",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "FailedCreateTarget",
		Message:            "uno de prueba",
	}

	if !reflect.DeepEqual(condition, replika.Status.Conditions) {
		replika.Status.Conditions = append([]metav1.Condition{}, condition)
		err := r.Status().Update(ctx, replika)
		if err != nil {
			log.Log.Error(err, "Failed to update Memcached status")
		}
	}
}


// NewReplikaCondition a set of default options for creating a Replika Condition.
func NewReplikaCondition(condType string, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetReplikaCondition returns the condition with the provided type.
func GetReplikaCondition(status replikav1alpha1.ReplikaStatus, condType string) *metav1.Condition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetReplikaCondition updates the replika to include the provided condition.
func SetReplikaCondition(ctx context.Context, replika *replikav1alpha1.Replika, condition metav1.Condition) {
	currentCond := GetReplikaCondition(replika.Status, condition.Type)

	if currentCond != nil && currentCond.Status == condition.Status &&
		currentCond.Reason == condition.Reason && currentCond.Message == condition.Message {
		updateReplikaCondition(ctx, replika, &condition)
		return
	}

	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}

	replika.Status.Conditions = append(filterOutCondition(replika.Status.Conditions, condition.Type), condition)

	if currentCond != nil {
		updateReplikaCondition(ctx, replika, currentCond)
	}

	updateReplikaCondition(ctx, replika, &condition)
}

// filterOutCondition returns an empty set of conditions with the provided type.
func filterOutCondition(conditions []metav1.Condition, condType string) []metav1.Condition {
	newConditions := make([]metav1.Condition, 0, len(conditions))
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// updateExternalSecretCondition updates the Replika conditions.
// Ref: https://github.com/external-secrets/external-secrets/blob/80545f4f183795ef193747fc959558c761b51c99/pkg/controllers/externalsecret/metrics.go#L53
// https://github.com/external-secrets/external-secrets/blob/80545f4f183795ef193747fc959558c761b51c99/pkg/controllers/externalsecret/util.go#L24
func updateReplikaCondition(ctx context.Context, replika *replikav1alpha1.Replika, condition *metav1.Condition) {
	switch condition.Type {
	case ReplikaDeleted:

	case ReplikaReady:

		// Toggle opposite Status to 0
		switch condition.Status {
		case metav1.ConditionFalse:

		case metav1.ConditionTrue:

		case metav1.ConditionUnknown:
			break
		default:
			break
		}

	default:
		break
	}
}