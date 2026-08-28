package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupJobTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

func newAuditServiceForJobsTest(db *gorm.DB) *AuditService {
	return NewAuditService(db, zap.NewNop())
}

func createPendingJob(t *testing.T, s *JobService) *models.ProvisioningJob {
	t.Helper()
	job, err := s.CreateProvisioningJob(
		uuid.New(), &uuid.UUID{}, datatypes.JSON(`{"vlan":100}`), datatypes.JSON(`{"vlan":50}`), uuid.New(),
	)
	require.NoError(t, err)
	return job
}

// ==================== Provisioning Job Tests ====================

func TestJobService_CreateProvisioningJob(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	userID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	ontID := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c9")
	templateID := uuid.MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430ca")

	configSnapshot := datatypes.JSON(`{"vlan": 100, "description": "Test ONT"}`)
	beforeSnapshot := datatypes.JSON(`{"vlan": 50, "description": "Old config"}`)

	job, err := service.CreateProvisioningJob(ontID, &templateID, configSnapshot, beforeSnapshot, userID)

	require.NoError(t, err)
	assert.Equal(t, ontID, job.ONTID)
	assert.Equal(t, templateID, *job.TemplateID)
	assert.Equal(t, models.ProvisioningStatusPending, job.Status)
	assert.Equal(t, configSnapshot, job.ConfigSnapshot)
	assert.Equal(t, beforeSnapshot, job.BeforeSnapshot)
	require.NotNil(t, job.CreatedBy)
	assert.Equal(t, userID, *job.CreatedBy)
	assert.False(t, job.CreatedAt.IsZero())
	assert.Nil(t, job.CompletedAt)
	assert.Nil(t, job.ErrorMessage)

	var jsonVal interface{}
	err = json.Unmarshal(job.ConfigSnapshot, &jsonVal)
	assert.NoError(t, err)
}

func TestJobService_CreateProvisioningJob_DuplicateRunning(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	userID := uuid.New()
	ontID := uuid.New()
	templateID := uuid.New()

	// Insert a running job directly to simulate an in-flight provisioning
	existing := &models.ProvisioningJob{
		ID:             uuid.New(),
		ONTID:          ontID,
		TemplateID:     &templateID,
		Status:         models.ProvisioningStatusRunning,
		ConfigSnapshot: datatypes.JSON(`{"existing": true}`),
		CreatedBy:      &userID,
	}
	require.NoError(t, db.Create(existing).Error)

	_, err := service.CreateProvisioningJob(ontID, &templateID, datatypes.JSON(`{}`), datatypes.JSON(`{}`), userID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "running")
	assert.Contains(t, err.Error(), ontID.String())
}

func TestJobService_CreateProvisioningJob_AllowedAfterTerminal(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	userID := uuid.New()
	ontID := uuid.New()
	templateID := uuid.New()

	// A finished job must not block a new one for the same ONT
	finished := &models.ProvisioningJob{
		ID:         uuid.New(),
		ONTID:      ontID,
		TemplateID: &templateID,
		Status:     models.ProvisioningStatusSuccess,
		CreatedBy:  &userID,
	}
	require.NoError(t, db.Create(finished).Error)

	_, err := service.CreateProvisioningJob(ontID, &templateID, datatypes.JSON(`{}`), datatypes.JSON(`{}`), userID)
	assert.NoError(t, err)
}

func TestJobService_GetProvisioningJob(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	t.Run("found", func(t *testing.T) {
		created := createPendingJob(t, service)
		found, err := service.GetProvisioningJob(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, models.ProvisioningStatusPending, found.Status)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetProvisioningJob(uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestJobService_UpdateStatusProvisioning_PendingToRunning(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	job := createPendingJob(t, service)
	err := service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil)
	assert.NoError(t, err)

	updated, err := service.GetProvisioningJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusRunning, updated.Status)
	assert.Nil(t, updated.CompletedAt)
}

func TestJobService_UpdateStatusProvisioning_RunningToSuccess(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	job := createPendingJob(t, service)
	require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil))

	err := service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusSuccess, nil)
	assert.NoError(t, err)

	updated, err := service.GetProvisioningJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusSuccess, updated.Status)
	assert.NotNil(t, updated.CompletedAt)
}

func TestJobService_UpdateStatusProvisioning_RunningToFailed(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	job := createPendingJob(t, service)
	require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil))

	errorMsg := "provisioning failed"
	err := service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &errorMsg)
	assert.NoError(t, err)

	updated, err := service.GetProvisioningJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusFailed, updated.Status)
	require.NotNil(t, updated.ErrorMessage)
	assert.Equal(t, errorMsg, *updated.ErrorMessage)
	assert.NotNil(t, updated.CompletedAt)
}

func TestJobService_UpdateStatusProvisioning_PendingToFailedBlocked(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	job := createPendingJob(t, service)
	errorMsg := "must go through running"
	err := service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &errorMsg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status transition")
	assert.Contains(t, err.Error(), models.ProvisioningStatusPending)
	assert.Contains(t, err.Error(), models.ProvisioningStatusFailed)

	// Status unchanged in DB
	updated, err := service.GetProvisioningJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusPending, updated.Status)
}

func TestJobService_UpdateStatusProvisioning_FailedToRolledBack(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	job := createPendingJob(t, service)
	require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil))
	errorMsg := "provisioning failed"
	require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &errorMsg))

	err := service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRolledBack, nil)
	assert.NoError(t, err)

	updated, err := service.GetProvisioningJob(job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusRolledBack, updated.Status)
}

func TestJobService_UpdateStatusProvisioning_InvalidTransitions(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	cases := []struct {
		name       string
		setupTo    string
		newStatus  string
		wantSubstr []string
	}{
		{
			name:       "pending_to_rolled_back",
			setupTo:    models.ProvisioningStatusPending,
			newStatus:  models.ProvisioningStatusRolledBack,
			wantSubstr: []string{models.ProvisioningStatusPending, models.ProvisioningStatusRolledBack},
		},
		{
			name:       "pending_to_success",
			setupTo:    models.ProvisioningStatusPending,
			newStatus:  models.ProvisioningStatusSuccess,
			wantSubstr: []string{models.ProvisioningStatusPending, models.ProvisioningStatusSuccess},
		},
		{
			name:       "success_to_running",
			setupTo:    models.ProvisioningStatusSuccess,
			newStatus:  models.ProvisioningStatusRunning,
			wantSubstr: []string{models.ProvisioningStatusSuccess, models.ProvisioningStatusRunning},
		},
		{
			name:       "success_to_rolled_back",
			setupTo:    models.ProvisioningStatusSuccess,
			newStatus:  models.ProvisioningStatusRolledBack,
			wantSubstr: []string{models.ProvisioningStatusSuccess, models.ProvisioningStatusRolledBack},
		},
		{
			name:       "rolled_back_to_running",
			setupTo:    models.ProvisioningStatusRolledBack,
			newStatus:  models.ProvisioningStatusRunning,
			wantSubstr: []string{models.ProvisioningStatusRolledBack, models.ProvisioningStatusRunning},
		},
		{
			name:       "invalid_status_value",
			setupTo:    models.ProvisioningStatusPending,
			newStatus:  "cancelled",
			wantSubstr: []string{"invalid status"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := createPendingJob(t, service)
			if tc.setupTo != models.ProvisioningStatusPending {
				require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil))
				if tc.setupTo != models.ProvisioningStatusRunning {
					if tc.setupTo == models.ProvisioningStatusRolledBack {
						msg := "err"
						require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &msg))
					}
					require.NoError(t, service.UpdateStatusProvisioning(job.ID, tc.setupTo, nil))
				}
			}

			err := service.UpdateStatusProvisioning(job.ID, tc.newStatus, nil)
			require.Error(t, err)
			for _, s := range tc.wantSubstr {
				assert.Contains(t, err.Error(), s)
			}
		})
	}
}

func TestJobService_UpdateStatusProvisioning_TransitionToRunningBlockedWhenDuplicateRunning(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	ontID := uuid.New()
	templateID := uuid.New()

	// One job already running on the ONT
	first, err := service.CreateProvisioningJob(ontID, &templateID, datatypes.JSON(`{}`), datatypes.JSON(`{}`), uuid.New())
	require.NoError(t, err)
	require.NoError(t, service.UpdateStatusProvisioning(first.ID, models.ProvisioningStatusRunning, nil))

	// Second pending job for the same ONT cannot start while first is running
	_, err = service.CreateProvisioningJob(ontID, &templateID, datatypes.JSON(`{}`), datatypes.JSON(`{}`), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running")
}

func TestJobService_UpdateStatusProvisioning_NotFound(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	err := service.UpdateStatusProvisioning(uuid.New(), models.ProvisioningStatusRunning, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestJobService_ListProvisioningJobsByONT(t *testing.T) {
	db := setupJobTestDB(t)
	service := NewJobService(db, newAuditServiceForJobsTest(db))

	targetONT := uuid.New()
	templateID := uuid.New()
	otherONT := uuid.New()

	for i := 0; i < 5; i++ {
		_, err := service.CreateProvisioningJob(targetONT, &templateID, datatypes.JSON(`{}`), datatypes.JSON(`{}`), uuid.New())
		require.NoError(t, err)
	}
	_, err := service.CreateProvisioningJob(otherONT, &templateID, datatypes.JSON(`{}`), datatypes.JSON(`{}`), uuid.New())
	require.NoError(t, err)

	t.Run("all jobs for ont", func(t *testing.T) {
		jobs, count, err := service.ListProvisioningJobsByONT(targetONT, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(5), count)
		assert.Len(t, jobs, 5)
		for _, j := range jobs {
			assert.Equal(t, targetONT, j.ONTID)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		jobs, count, err := service.ListProvisioningJobsByONT(targetONT, 2, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(5), count)
		assert.Len(t, jobs, 2)
	})

	t.Run("offset beyond total", func(t *testing.T) {
		jobs, count, err := service.ListProvisioningJobsByONT(targetONT, 10, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(5), count)
		assert.Empty(t, jobs)
	})
}

func TestJobService_AuditLogging(t *testing.T) {
	db := setupJobTestDB(t)
	audit := NewAuditService(db, zap.NewNop())
	service := NewJobService(db, audit)

	userID := uuid.New()

	t.Run("create provisioning job logs audit", func(t *testing.T) {
		job, err := service.CreateProvisioningJob(
			uuid.New(), &uuid.UUID{}, datatypes.JSON(`{}`), datatypes.JSON(`{}`), userID,
		)
		require.NoError(t, err)

		var count int64
		db.Model(&models.AuditLog{}).
			Where("action = ? AND resource_type = ? AND resource_id = ?", "create", "provisioning_job", job.ID).
			Count(&count)
		assert.Equal(t, int64(1), count)
	})

	t.Run("update provisioning job status logs audit", func(t *testing.T) {
		job, err := service.CreateProvisioningJob(
			uuid.New(), &uuid.UUID{}, datatypes.JSON(`{}`), datatypes.JSON(`{}`), userID,
		)
		require.NoError(t, err)
		require.NoError(t, service.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil))

		var count int64
		db.Model(&models.AuditLog{}).
			Where("action = ? AND resource_type = ? AND resource_id = ?", "update", "provisioning_job", job.ID).
			Count(&count)
		assert.Equal(t, int64(1), count)
	})

}
