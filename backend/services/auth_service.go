package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"mcloud/config"
	"mcloud/models"
	"mcloud/repositories"
	"mcloud/utils"

	"gorm.io/gorm"
)

type RegisterInput struct {
	Username string
	Password string
	Nickname string
}

type LoginInput struct {
	Username string
	Password string
}

type AuthUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

type LoginOutput struct {
	Token string   `json:"token"`
	User  AuthUser `json:"user"`
}

type ProfileOutput struct {
	ID           uint      `json:"id"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	StorageQuota int64     `json:"storage_quota"`
	StorageUsed  int64     `json:"storage_used"`
	RootFolderID uint      `json:"root_folder_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuthService interface {
	Register(ctx context.Context, in RegisterInput) (AuthUser, error)
	Login(ctx context.Context, in LoginInput) (LoginOutput, error)
	GetProfile(ctx context.Context, userID uint) (ProfileOutput, error)
}

type authService struct {
	txManager TxManager
	users     repositories.UserRepository
	resolver  folderResolver
}

func NewAuthService(txManager TxManager, users repositories.UserRepository, folders repositories.FolderRepository) AuthService {
	return &authService{txManager: txManager, users: users, resolver: folderResolver{folders: folders}}
}

func (s *authService) Register(ctx context.Context, in RegisterInput) (AuthUser, error) {
	count, err := s.users.CountByUsername(ctx, in.Username)
	if err != nil {
		return AuthUser{}, newAppError(http.StatusInternalServerError, "failed to check username", err)
	}
	if count > 0 {
		return AuthUser{}, newAppError(http.StatusBadRequest, "username already exists", nil)
	}

	hashedPassword, err := utils.HashPassword(in.Password)
	if err != nil {
		return AuthUser{}, newAppError(http.StatusInternalServerError, "failed to hash password", err)
	}

	user := models.User{
		Username:     in.Username,
		Password:     hashedPassword,
		Nickname:     in.Nickname,
		StorageQuota: config.AppConfig.Storage.DefaultUserQuota,
	}

	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := s.users.Create(ctx, tx, &user); err != nil {
			return err
		}
		_, err := s.resolver.getOrCreateUserRootFolder(ctx, tx, user.ID)
		return err
	})
	if err != nil {
		return AuthUser{}, newAppError(http.StatusInternalServerError, "failed to create user", err)
	}

	return AuthUser{ID: user.ID, Username: user.Username, Nickname: user.Nickname}, nil
}

func (s *authService) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	user, err := s.users.GetByUsername(ctx, nil, in.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LoginOutput{}, newAppError(http.StatusUnauthorized, "invalid username or password", nil)
		}
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "failed to query user", err)
	}

	if !utils.CheckPassword(in.Password, user.Password) {
		return LoginOutput{}, newAppError(http.StatusUnauthorized, "invalid username or password", nil)
	}

	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "failed to generate token", err)
	}

	return LoginOutput{
		Token: token,
		User:  AuthUser{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
	}, nil
}

func (s *authService) GetProfile(ctx context.Context, userID uint) (ProfileOutput, error) {
	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ProfileOutput{}, newAppError(http.StatusNotFound, "user not found", nil)
		}
		return ProfileOutput{}, newAppError(http.StatusInternalServerError, "failed to query user", err)
	}

	rootFolder, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, user.ID)
	if err != nil {
		return ProfileOutput{}, newAppError(http.StatusInternalServerError, "failed to load root folder", err)
	}

	return ProfileOutput{
		ID:           user.ID,
		Username:     user.Username,
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		StorageQuota: user.StorageQuota,
		StorageUsed:  user.StorageUsed,
		RootFolderID: rootFolder.ID,
		CreatedAt:    user.CreatedAt,
	}, nil
}
