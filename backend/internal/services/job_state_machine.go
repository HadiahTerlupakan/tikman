package services

import (
	"github.com/tikman/olt-provisioning/internal/models"
)

func isValidProvisioningStatus(status string) bool {
	switch status {
	case models.ProvisioningStatusPending, models.ProvisioningStatusRunning,
		models.ProvisioningStatusSuccess, models.ProvisioningStatusFailed,
		models.ProvisioningStatusRolledBack:
		return true
	}
	return false
}

// isValidProvisioningTransition validates a provisioning job state transition.
// State machine: pending -> running -> success | failed, and failed -> rolled_back.
// success and rolled_back are terminal.
func isValidProvisioningTransition(from, to string) bool {
	allowed := map[string][]string{
		models.ProvisioningStatusPending:    {models.ProvisioningStatusRunning},
		models.ProvisioningStatusRunning:    {models.ProvisioningStatusSuccess, models.ProvisioningStatusFailed},
		models.ProvisioningStatusFailed:     {models.ProvisioningStatusRolledBack},
		models.ProvisioningStatusSuccess:    nil, // terminal
		models.ProvisioningStatusRolledBack: nil, // terminal
	}
	toStates, ok := allowed[from]
	if !ok {
		return false
	}
	for _, candidate := range toStates {
		if candidate == to {
			return true
		}
	}
	return false
}

func isProvisioningTerminal(status string) bool {
	return status == models.ProvisioningStatusSuccess ||
		status == models.ProvisioningStatusFailed ||
		status == models.ProvisioningStatusRolledBack
}

func isValidBatchStatus(status string) bool {
	switch status {
	case models.BatchStatusPending, models.BatchStatusRunning,
		models.BatchStatusSuccess, models.BatchStatusFailed,
		models.BatchStatusPartialRollback:
		return true
	}
	return false
}

// isValidBatchTransition validates a batch job state transition.
// State machine: pending -> running -> success | failed | partial_rollback.
// success, failed, and partial_rollback are terminal.
func isValidBatchTransition(from, to string) bool {
	allowed := map[string][]string{
		models.BatchStatusPending: {models.BatchStatusRunning},
		models.BatchStatusRunning: {
			models.BatchStatusSuccess,
			models.BatchStatusFailed,
			models.BatchStatusPartialRollback,
		},
		models.BatchStatusSuccess:         nil, // terminal
		models.BatchStatusFailed:          nil, // terminal
		models.BatchStatusPartialRollback: nil, // terminal
	}
	toStates, ok := allowed[from]
	if !ok {
		return false
	}
	for _, candidate := range toStates {
		if candidate == to {
			return true
		}
	}
	return false
}

func isBatchTerminal(status string) bool {
	return status == models.BatchStatusSuccess ||
		status == models.BatchStatusFailed ||
		status == models.BatchStatusPartialRollback
}
