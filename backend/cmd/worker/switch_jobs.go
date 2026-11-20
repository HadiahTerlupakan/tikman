package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// runNextSwitchJob claims one switch job and runs it, reporting whether there
// was work.
func runNextSwitchJob(rt *workerRuntime) bool {
	job, err := rt.switchJobs.Claim(rt.id)
	if err != nil {
		rt.logger.Error("Failed to claim a switch poll job", zap.Error(err))
		time.Sleep(errorWait)
		return true
	}
	if job == nil {
		return false
	}

	sw, err := rt.switches.Get(job.SwitchID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// A database error says nothing about the switch; completing the job
		// would hide the error and wait a whole interval before trying again.
		rt.logger.Warn("Failed to load the switch for its poll job",
			zap.String("switch_id", job.SwitchID.String()), zap.Error(err))
		failSwitchJob(rt, job, err)
		return true
	}
	if err != nil || !sw.Enabled {
		// Gone (its jobs follow by foreign key) or switched off: nothing to
		// read, and not a failure to back off over.
		if completeErr := rt.switchJobs.Complete(job, 0); completeErr != nil {
			rt.logger.Error("Failed to release a switch job", zap.Error(completeErr))
		}
		return true
	}

	started := time.Now()
	if err := runSwitchJob(rt, job, sw); err != nil {
		rt.logger.Warn("Switch poll job failed",
			zap.String("switch", sw.Name), zap.String("kind", string(job.Kind)), zap.Error(err))
		failSwitchJob(rt, job, err)
		return true
	}
	if err := rt.switchJobs.Complete(job, time.Since(started)); err != nil {
		rt.logger.Error("Failed to record a switch job completion", zap.Error(err))
	}
	return true
}

func failSwitchJob(rt *workerRuntime, job *models.SwitchPollJob, cause error) {
	if err := rt.switchJobs.Fail(job, cause); err != nil {
		rt.logger.Error("Failed to record a switch job failure", zap.Error(err))
	}
}

func runSwitchJob(rt *workerRuntime, job *models.SwitchPollJob, sw *models.Switch) error {
	switch job.Kind {
	case models.SwitchPollKindInterfaces:
		return rt.switchPoll.PollInterfaces(sw, time.Now())
	case models.SwitchPollKindInventory:
		return rt.switchPoll.Inventory(sw, time.Now())
	case models.SwitchPollKindBackup:
		return rt.switchPoll.Backup(sw, time.Now())
	default:
		return fmt.Errorf("unknown switch poll kind %q", job.Kind)
	}
}
