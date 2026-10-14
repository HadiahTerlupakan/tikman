package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// A database error says nothing about the router; completing the job
		// would hide the error and wait a whole interval before trying again.
		rt.logger.Warn("Failed to load the router for its poll job",
			zap.String("router_id", job.RouterID.String()), zap.Error(err))
		failRouterJob(rt, job, err)
		return true
	}
	if err != nil || !router.Enabled {
		// Gone (its jobs follow by foreign key) or switched off: either way
		// there is nothing to read, and that is not a failure to back off over.
		if completeErr := rt.routerJobs.Complete(job, 0); completeErr != nil {
			rt.logger.Error("Failed to release a router job", zap.Error(completeErr))
		}
		return true
	}

	started := time.Now()
	settleRouterJob(rt, job, router, runRouterJob(rt, job, router), time.Since(started))
	return true
}

// settleRouterJob records how a run ended. An unread route table is not a
// failure: discovery still refreshed the interfaces, and failing the job would
// push it from 6 h to 24 h on a router whose route read keeps timing out.
func settleRouterJob(rt *workerRuntime, job *models.RouterPollJob, router *models.MikrotikRouter, err error, took time.Duration) {
	if errors.Is(err, services.ErrStaticRoutesUnread) {
		rt.logger.Warn("Router discovery could not read static routes",
			zap.String("router", router.Name), zap.String("router_id", router.ID.String()), zap.Error(err))
		err = nil
	}
	if err != nil {
		rt.logger.Warn("Router poll job failed",
			zap.String("router", router.Name), zap.String("kind", string(job.Kind)), zap.Error(err))
		failRouterJob(rt, job, err)
		return
	}
	if err := rt.routerJobs.Complete(job, took); err != nil {
		rt.logger.Error("Failed to record a router job completion", zap.Error(err))
	}
}

func failRouterJob(rt *workerRuntime, job *models.RouterPollJob, cause error) {
	if err := rt.routerJobs.Fail(job, cause); err != nil {
		rt.logger.Error("Failed to record a router job failure", zap.Error(err))
	}
}

func runRouterJob(rt *workerRuntime, job *models.RouterPollJob, router *models.MikrotikRouter) error {
	switch job.Kind {
	case models.RouterPollKindInterfaces:
		return rt.routerPoll.PollInterfaces(router, time.Now())
	case models.RouterPollKindBGP:
		return rt.routerPoll.PollBGP(router, time.Now())
	case models.RouterPollKindDiscovery:
		return rt.routerPoll.Discover(router)
	case models.RouterPollKindRoutes:
		return rt.routerPoll.ReadRoutes(router, time.Now())
	default:
		return fmt.Errorf("unknown router poll kind %q", job.Kind)
	}
}
