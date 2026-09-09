package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/auth"

	"gorm.io/gorm"
)

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) auth.Repository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindByEmail(ctx context.Context, email string) (*auth.User, error) {
	var user auth.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindByID(ctx context.Context, id uint) (*auth.User, error) {
	var user auth.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) Create(ctx context.Context, user *auth.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *AuthRepository) CreateWithBootstrapRole(ctx context.Context, user *auth.User) (string, error) {
	var assignedRole string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire transaction-level lock or count inside tx
		var count int64
		// Query with locking if supported, or count inside tx
		if err := tx.Model(&auth.User{}).Count(&count).Error; err != nil {
			return err
		}

		if count == 0 {
			assignedRole = "owner"
		} else {
			assignedRole = "sales"
		}

		user.Role = assignedRole
		return tx.Create(user).Error
	})
	return assignedRole, err
}

func (r *AuthRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&auth.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AuthRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&auth.User{}).Count(&count).Error
	return count, err
}

func (r *AuthRepository) FindAll(ctx context.Context) ([]*auth.User, error) {
	var users []*auth.User
	err := r.db.WithContext(ctx).Order("id asc").Find(&users).Error
	return users, err
}

func (r *AuthRepository) Update(ctx context.Context, user *auth.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *AuthRepository) UpdateStatus(ctx context.Context, id uint, isActive bool) error {
	return r.db.WithContext(ctx).Model(&auth.User{}).Where("id = ?", id).Update("is_active", isActive).Error
}

func (r *AuthRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&auth.User{}, id).Error
}

