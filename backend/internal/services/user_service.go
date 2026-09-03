package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// Create adds a user. An empty initials falls back to deriveInitials(username)
// — the field is optional precisely so most admins never have to think about it.
func (s *UserService) Create(username, email, password, initials string, role models.UserRole) (*models.User, error) {
	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	mark := normalizeInitials(initials)
	if mark == "" {
		mark = deriveInitials(username)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		Initials:     mark,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *UserService) GetByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *UserService) GetByUsername(username string) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, "username = ?", username).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *UserService) List() ([]models.User, error) {
	var users []models.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}

// Update applies a partial change set. An initials value that normalizes to
// empty means "clear it" in the request, but the column is not nullable, so
// it is resolved back to deriveInitials on the user's current username rather
// than stored blank.
func (s *UserService) Update(id uuid.UUID, updates map[string]interface{}) error {
	if password, ok := updates["password"].(string); ok {
		hash, err := utils.HashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		updates["password_hash"] = hash
		delete(updates, "password")
	}

	if raw, ok := updates["initials"].(string); ok {
		mark := normalizeInitials(raw)
		if mark == "" {
			user, err := s.GetByID(id)
			if err != nil {
				return err
			}
			mark = deriveInitials(user.Username)
		}
		updates["initials"] = mark
	}

	if err := s.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (s *UserService) Delete(id uuid.UUID) error {
	if err := s.db.Delete(&models.User{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

func (s *UserService) VerifyPassword(user *models.User, password string) error {
	return utils.ComparePassword(user.PasswordHash, password)
}
