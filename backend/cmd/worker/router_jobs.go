package main

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

// runNextRouterJob claims one router job and runs it, reporting whether there
// was work.
func runNextRouterJob(rt *workerRuntime) bool {
	job, err := rt.routerJobs.Claim(rt.id)
	if err != nil {
		rt.logger.Error("Failed to claim a router poll job", zap.Error(err))
		time.Sleep(errorWait)
		return true
	}
	if job == nil {
		return false
	}

	router, err := rt.routers.Get(job.RouterID)
	if err != nil || !router.Enabled {
		// Gone (its jobs follow by foreign key) or switched off: either way
		// there is nothing to read, and that is not a failure to back off over.
		if completeErr := rt.routerJobs.Complete(job, 0); completeErr != nil {
			rt.logger.Error("Failed to release a router job", zap.Error(completeErr))
		}
		return true
	}

	started := time.Now()
	if err := runRouterJob(rt, job, router); err != nil {
		rt.logger.Warn("Router poll job failed",
			zap.String("router", router.Name), zap.String("kind", string(job.Kind)), zap.Error(err))
		if failErr := rt.routerJobs.Fail(job, err); failErr != nil {
			rt.logger.Error("Failed to record a router job failure", zap.Error(failErr))
		}
		return true
	}
	if err := rt.routerJobs.Complete(job, time.Since(started)); err != nil {
		rt.logger.Error("Failed to record a router job completion", zap.Error(err))
	}
	return true
}

func runRouterJob(rt *workerRuntime, job *models.RouterPollJob, router *models.MikrotikRouter) error {
	switch job.Kind {
	case models.RouterPollKindInterfaces:
		return rt.routerPoll.PollInterfaces(router, time.Now())
	case models.RouterPollKindBGP:
		return rt.routerPoll.PollBGP(router, time.Now())
	case models.RouterPollKindDiscovery:
		return rt.routerPoll.Discover(router)
	default:
		return fmt.Errorf("unknown router poll kind %q", job.Kind)
	}
}
