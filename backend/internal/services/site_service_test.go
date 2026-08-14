package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSiteTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Site{}, &models.OLT{})
	require.NoError(t, err)

	return db
}

func TestSiteService_Create(t *testing.T) {
	db := setupSiteTestDB(t)
	service := NewSiteService(db)

	t.Run("success", func(t *testing.T) {
		site, err := service.Create("HQ Office", "123 Main St", "Headquarters location")
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, site.ID)
		assert.Equal(t, "HQ Office", site.Name)
		assert.Equal(t, "123 Main St", site.Location)
		assert.Equal(t, "Headquarters location", site.Description)
		assert.False(t, site.CreatedAt.IsZero())
		assert.False(t, site.UpdatedAt.IsZero())
	})

	t.Run("empty description", func(t *testing.T) {
		site, err := service.Create("Branch Office", "456 Oak Ave", "")
		require.NoError(t, err)
		assert.Equal(t, "Branch Office", site.Name)
		assert.Equal(t, "", site.Description)
	})
}

func TestSiteService_GetByID(t *testing.T) {
	db := setupSiteTestDB(t)
	service := NewSiteService(db)

	t.Run("success", func(t *testing.T) {
		created, err := service.Create("Test Site", "789 Elm St", "Test")
		require.NoError(t, err)

		found, err := service.GetByID(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "Test Site", found.Name)
		assert.Equal(t, "789 Elm St", found.Location)
		assert.Equal(t, "Test", found.Description)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetByID(uuid.New())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "site not found")
	})
}

func TestSiteService_List(t *testing.T) {
	db := setupSiteTestDB(t)
	service := NewSiteService(db)

	t.Run("empty list", func(t *testing.T) {
		sites, err := service.List()
		require.NoError(t, err)
		assert.Empty(t, sites)
	})

	t.Run("multiple sites", func(t *testing.T) {
		_, err := service.Create("Site 1", "Location 1", "Desc 1")
		require.NoError(t, err)
		_, err = service.Create("Site 2", "Location 2", "Desc 2")
		require.NoError(t, err)

		sites, err := service.List()
		require.NoError(t, err)
		assert.Len(t, sites, 2)
	})
}

func TestSiteService_Update(t *testing.T) {
	db := setupSiteTestDB(t)
	service := NewSiteService(db)

	t.Run("success", func(t *testing.T) {
		site, err := service.Create("Old Name", "Old Location", "Old Desc")
		require.NoError(t, err)

		updates := map[string]interface{}{
			"name":        "New Name",
			"location":    "New Location",
			"description": "New Desc",
		}
		err = service.Update(site.ID, updates)
		require.NoError(t, err)

		updated, err := service.GetByID(site.ID)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updated.Name)
		assert.Equal(t, "New Location", updated.Location)
		assert.Equal(t, "New Desc", updated.Description)
	})

	t.Run("partial update", func(t *testing.T) {
		site, err := service.Create("Original", "Original Loc", "Original Desc")
		require.NoError(t, err)

		updates := map[string]interface{}{
			"name": "Updated Name",
		}
		err = service.Update(site.ID, updates)
		require.NoError(t, err)

		updated, err := service.GetByID(site.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "Original Loc", updated.Location)
		assert.Equal(t, "Original Desc", updated.Description)
	})
}

func TestSiteService_Delete(t *testing.T) {
	db := setupSiteTestDB(t)
	service := NewSiteService(db)

	t.Run("success", func(t *testing.T) {
		site, err := service.Create("To Delete", "Location", "Desc")
		require.NoError(t, err)

		err = service.Delete(site.ID)
		require.NoError(t, err)

		_, err = service.GetByID(site.ID)
		assert.Error(t, err)
	})

	t.Run("non-existent", func(t *testing.T) {
		err := service.Delete(uuid.New())
		assert.NoError(t, err) // GORM doesn't error on delete of non-existent records
	})
}
