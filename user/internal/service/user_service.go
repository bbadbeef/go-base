package service

import (
	"fmt"

	"github.com/bbadbeef/go-base/user/internal/model"
	"github.com/bbadbeef/go-base/user/internal/repository"
)

// UserService 用户服务
type UserService struct {
	userRepo *repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(id int64) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

// GetUserProfile 获取用户公开信息
func (s *UserService) GetUserProfile(id int64) (*model.UserProfile, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	return user.ToProfile(), nil
}

// UpdateProfile 更新用户信息
func (s *UserService) UpdateProfile(userID int64, req *model.UpdateProfileRequest) (*model.User, error) {
	// 获取用户
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.Nickname != nil {
		if err := s.validateNickname(*req.Nickname); err != nil {
			return nil, err
		}
		user.Nickname = *req.Nickname
	}

	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}

	if req.Email != nil {
		if err := s.validateEmail(*req.Email); err != nil {
			return nil, err
		}
		user.Email = *req.Email
	}

	if req.Gender != nil {
		if *req.Gender < 0 || *req.Gender > 2 {
			return nil, fmt.Errorf("invalid gender value")
		}
		user.Gender = *req.Gender
	}

	if req.Birthday != nil {
		user.Birthday = req.Birthday
	}

	if req.Signature != nil {
		if len(*req.Signature) > 255 {
			return nil, fmt.Errorf("signature too long")
		}
		user.Signature = *req.Signature
	}

	user.UpdatedAt = model.NowMillis()

	// 保存更新
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return user, nil
}

// validateNickname 验证昵称
func (s *UserService) validateNickname(nickname string) error {
	if nickname == "" {
		return fmt.Errorf("nickname cannot be empty")
	}

	if len(nickname) > 50 {
		return fmt.Errorf("nickname too long")
	}

	return nil
}

// validateEmail 验证邮箱
func (s *UserService) validateEmail(email string) error {
	if email == "" {
		return nil
	}

	// 简单的邮箱格式验证
	if len(email) > 100 {
		return fmt.Errorf("email too long")
	}

	return nil
}
