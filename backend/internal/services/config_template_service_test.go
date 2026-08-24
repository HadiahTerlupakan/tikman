package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTemplateTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

func newAuditServiceForTest(db *gorm.DB) *AuditService {
	// Use zap logger that discards output for tests
	logger := zap.NewNop()
	return NewAuditService(db, logger)
}

func TestConfigTemplateService_Create(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("success", func(t *testing.T) {
		configFields := map[string]interface{}{
			"vlan":        100,
			"description": "Test template",
			"enabled":     true,
		}

		template, err := service.Create(
			"test-template",
			"A test template",
			models.VendorZTE,
			configFields,
			false,
			uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		)

		require.NoError(t, err)
		assert.Equal(t, "test-template", template.Name)
		assert.Equal(t, "A test template", template.Description)
		assert.Equal(t, models.VendorZTE, template.Vendor)
		assert.False(t, template.IsDefault)
		assert.NotEqual(t, uuid.Nil, template.ID)
		assert.False(t, template.CreatedAt.IsZero())

		var jsonVal interface{}
		err = json.Unmarshal(template.ConfigFields, &jsonVal)
		assert.NoError(t, err)
	})

	t.Run("empty description allowed", func(t *testing.T) {
		template, err := service.Create(
			"minimal-template",
			"",
			models.VendorHSGQ,
			map[string]interface{}{},
			true,
			uuid.Nil,
		)

		require.NoError(t, err)
		assert.Equal(t, "", template.Description)
		assert.True(t, template.IsDefault)
	})
}

func TestConfigTemplateService_Create_Validation(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("name too short", func(t *testing.T) {
		_, err := service.Create("ab", "Too short", models.VendorZTE, nil, false, uuid.Nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be between 3 and 100 characters")
	})

	t.Run("name too long", func(t *testing.T) {
		longName := make([]rune, 101)
		for i := range longName {
			longName[i] = 'x'
		}
		_, err := service.Create(string(longName), "Too long", models.VendorZTE, nil, false, uuid.Nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be between 3 and 100 characters")
	})

	t.Run("duplicate name", func(t *testing.T) {
		config := map[string]interface{}{"key": "value"}
		_, err := service.Create("unique-name", "First", models.VendorZTE, config, false, uuid.Nil)
		require.NoError(t, err)

		_, err = service.Create("unique-name", "Duplicate", models.VendorZTE, config, false, uuid.Nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be unique")
	})

	t.Run("invalid vendor zte", func(t *testing.T) {
		_, err := service.Create("bad-vendor", "Invalid", "nonexistent_vendor", nil, false, uuid.Nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vendor must be one of")
	})

	t.Run("invalid vendor hsgq", func(t *testing.T) {
		_, err := service.Create("bad-hsgq", "Invalid", "unknown_hsgq_variant", nil, false, uuid.Nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vendor must be one of")
	})
}

func TestConfigTemplateService_Create_DefaultHandling(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("clear existing default when setting new default same vendor", func(t *testing.T) {
		// Create first default
		first, err := service.Create(
			"first-default",
			"First default",
			models.VendorZTE,
			map[string]interface{}{},
			true,
			uuid.Nil,
		)
		require.NoError(t, err)
		require.True(t, first.IsDefault)

		// Create second default for same vendor
		second, err := service.Create(
			"second-default",
			"Second default",
			models.VendorZTE,
			map[string]interface{}{},
			true,
			uuid.Nil,
		)
		require.NoError(t, err)

		// Second should now be the only default for ZTE
		assert.True(t, second.IsDefault)

		// First should no longer be default - refetch from DB
		refetched, err := service.GetByID(first.ID)
		require.NoError(t, err)
		assert.False(t, refetched.IsDefault)
	})

	t.Run("different vendors can each have defaults", func(t *testing.T) {
		zte, err := service.Create(
			"zte-default",
			"ZTE default",
			models.VendorZTE,
			map[string]interface{}{},
			true,
			uuid.Nil,
		)
		require.NoError(t, err)

		hsgq, err := service.Create(
			"hsgq-default",
			"HSGQ default",
			models.VendorHSGQ,
			map[string]interface{}{},
			true,
			uuid.Nil,
		)
		require.NoError(t, err)

		// Both should remain defaults (same vendor is key to clearing)
		assert.True(t, zte.IsDefault)
		assert.True(t, hsgq.IsDefault)

		// Verify in database
		var ztes []models.ConfigTemplate
		require.NoError(t, db.Where("vendor = ? AND is_default = ?", models.VendorZTE, true).Find(&ztes).Error)
		assert.Len(t, ztes, 1)
		assert.Equal(t, "zte-default", ztes[0].Name)

		var hsgqs []models.ConfigTemplate
		require.NoError(t, db.Where("vendor = ? AND is_default = ?", models.VendorHSGQ, true).Find(&hsgqs).Error)
		assert.Len(t, hsgqs, 1)
		assert.Equal(t, "hsgq-default", hsgqs[0].Name)
	})
}

func TestConfigTemplateService_GetByID(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("found", func(t *testing.T) {
		created, err := service.Create(
			"find-me",
			"Found template",
			models.VendorZTE,
			map[string]interface{}{},
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		found, err := service.GetByID(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "find-me", found.Name)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetByID(uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config template not found")
	})
}

func TestConfigTemplateService_List(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("empty list", func(t *testing.T) {
		templates, err := service.List()
		require.NoError(t, err)
		assert.Empty(t, templates)
	})

	t.Run("multiple templates", func(t *testing.T) {
		_, err := service.Create("template-a", "A", models.VendorZTE, nil, false, uuid.Nil)
		require.NoError(t, err)

		_, err = service.Create("template-b", "B", models.VendorHSGQ, nil, false, uuid.Nil)
		require.NoError(t, err)

		_, err = service.Create("template-c", "C", models.VendorZTE, nil, false, uuid.Nil)
		require.NoError(t, err)

		templates, err := service.List()
		require.NoError(t, err)
		assert.Len(t, templates, 3)
	})
}

func TestConfigTemplateService_Update(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("success update all fields", func(t *testing.T) {
		original, err := service.Create(
			"original",
			"Original desc",
			models.VendorZTE,
			map[string]interface{}{"old": "field"},
			true,
			uuid.Nil,
		)
		require.NoError(t, err)

		newConfig := map[string]interface{}{"new": "field"}
		updated, err := service.Update(
			original.ID,
			"renamed-template",
			"Updated description",
			models.VendorHSGQ,
			newConfig,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		assert.Equal(t, "renamed-template", updated.Name)
		assert.Equal(t, "Updated description", updated.Description)
		assert.Equal(t, models.VendorHSGQ, updated.Vendor)
		assert.False(t, updated.IsDefault)

		var jsonVal interface{}
		err = json.Unmarshal(updated.ConfigFields, &jsonVal)
		assert.NoError(t, err)
	})

	t.Run("update with name conflict", func(t *testing.T) {
		_, err := service.Create(
			"conflict-target",
			"First template",
			models.VendorZTE,
			nil,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		second, err := service.Create(
			"second",
			"Second template",
			models.VendorZTE,
			nil,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		// Try to rename second to match first's name
		_, err = service.Update(second.ID, "conflict-target", "New desc", models.VendorZTE, nil, false, uuid.Nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be unique")
	})

	t.Run("partial update", func(t *testing.T) {
		template, err := service.Create(
			"partial-test",
			"Original description",
			models.VendorZTE,
			map[string]interface{}{"keep": "this"},
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		_, err = service.Update(
			template.ID,
			"partial-test",
			"Only description changed",
			models.VendorZTE,
			map[string]interface{}{"modify": "desc"},
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		updated, err := service.GetByID(template.ID)
		require.NoError(t, err)
		assert.Equal(t, "Only description changed", updated.Description)
	})

	t.Run("update invalid vendor", func(t *testing.T) {
		template, err := service.Create(
			"invalid-update",
			"Desc",
			models.VendorZTE,
			nil,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		_, err = service.Update(
			template.ID,
			"invalid-update",
			"Bad vendor",
			"not_a_vendor",
			nil,
			false,
			uuid.Nil,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vendor must be one of")
	})

	t.Run("update non-existent template", func(t *testing.T) {
		_, err := service.Update(
			uuid.New(),
			"name",
			"Desc",
			models.VendorZTE,
			nil,
			false,
			uuid.Nil,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config template not found")
	})
}

func TestConfigTemplateService_Delete(t *testing.T) {
	db := setupTemplateTestDB(t)
	audit := newAuditServiceForTest(db)
	service := NewConfigTemplateService(db, audit)

	t.Run("delete unreferenced success", func(t *testing.T) {
		template, err := service.Create(
			"to-delete",
			"Delete me",
			models.VendorZTE,
			nil,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		err = service.Delete(template.ID)
		require.NoError(t, err)

		_, err = service.GetByID(template.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config template not found")
	})

	t.Run("delete blocked by provisioning job reference", func(t *testing.T) {
		template, err := service.Create(
			"referenced",
			"I am referenced",
			models.VendorZTE,
			nil,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		jobID := uuid.New()
		userID := uuid.New()

		err = db.Create(&models.ProvisioningJob{
			ID:         jobID,
			ONTID:      uuid.New(),
			TemplateID: &template.ID,
			Status:     models.ProvisioningStatusPending,
			CreatedBy:  &userID,
		}).Error
		require.NoError(t, err)

		err = service.Delete(template.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provisioning job")
	})

	t.Run("delete blocked by batch job reference", func(t *testing.T) {
		template, err := service.Create(
			"batch-referenced",
			"I am batch referenced",
			models.VendorHSGQ,
			nil,
			false,
			uuid.Nil,
		)
		require.NoError(t, err)

		batchID := uuid.New()
		userID := uuid.New()
		ontID := uuid.New()

		err = db.Create(&models.BatchJob{
			ID:         batchID,
			TemplateID: template.ID,
			ONTIDs:     []uuid.UUID{ontID},
			Status:     models.BatchStatusPending,
			CreatedBy:  &userID,
		}).Error
		require.NoError(t, err)

		err = service.Delete(template.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch job")
	})

	t.Run("delete non-existent template", func(t *testing.T) {
		err := service.Delete(uuid.New())
		// Should handle gracefully or error with not found
		assert.Error(t, err)
	})
}

func TestConfigTemplateService_GetDefaultByVendor(t *testing.T) {
	t.Run("found default", func(t *testing.T) {
		db := setupTemplateTestDB(t)
		audit := newAuditServiceForTest(db)
		service := NewConfigTemplateService(db, audit)

		defaultTemplate, err := service.Create(
			"default-tpl",
			"The default",
			models.VendorZTE,
			nil,
			true,
			uuid.Nil,
		)
		require.NoError(t, err)

		found, err := service.GetDefaultByVendor(models.VendorZTE)
		require.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, defaultTemplate.ID, found.ID)
		assert.True(t, found.IsDefault)
	})

	t.Run("not found returns nil without error", func(t *testing.T) {
		db := setupTemplateTestDB(t)
		audit := newAuditServiceForTest(db)
		service := NewConfigTemplateService(db, audit)

		found, err := service.GetDefaultByVendor(models.VendorZTE)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("different vendor has no default", func(t *testing.T) {
		db := setupTemplateTestDB(t)
		audit := newAuditServiceForTest(db)
		service := NewConfigTemplateService(db, audit)

		// Create a default for HSGQ
		_, err := service.Create(
			"hsgq-default",
			"HSGQ def",
			models.VendorHSGQ,
			nil,
			true,
			uuid.Nil,
		)
		require.NoError(t, err)

		// Query for ZTE should return nil
		found, err := service.GetDefaultByVendor(models.VendorZTE)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}
