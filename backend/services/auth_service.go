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

// RegisterInput 定义注册请求参数。
type RegisterInput struct {
	// Username 为登录唯一标识。
	Username string
	// Password 为明文密码，服务内会做哈希后持久化。
	Password string
	// Nickname 为用户展示名。
	Nickname string
}

// LoginInput 定义登录请求参数。
type LoginInput struct {
	// Username 为登录账号。
	Username string
	// Password 为登录密码明文。
	Password string
}

// AuthUser 为登录态中的简化用户信息。
type AuthUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// LoginOutput 为登录/注册成功后的返回体。
type LoginOutput struct {
	Token string   `json:"token"`
	User  AuthUser `json:"user"`
}

// ProfileOutput 为个人资料查询返回体。
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

// AuthService 定义认证相关用例。
type AuthService interface {
	// Register 注册新用户并返回登录态。
	Register(ctx context.Context, in RegisterInput) (LoginOutput, error)
	// Login 校验账号密码并返回登录态。
	Login(ctx context.Context, in LoginInput) (LoginOutput, error)
	// GetProfile 查询当前用户信息。
	GetProfile(ctx context.Context, userID uint) (ProfileOutput, error)
}

// authService 为 AuthService 的默认实现。
type authService struct {
	txManager TxManager
	users     repositories.UserRepository
	resolver  folderResolver
}

// NewAuthService 创建认证服务实例。
func NewAuthService(txManager TxManager, users repositories.UserRepository, folders repositories.FolderRepository) AuthService {
	return &authService{txManager: txManager, users: users, resolver: folderResolver{folders: folders}}
}

// Register 注册新用户并返回登录凭证与基础用户信息。
func (s *authService) Register(ctx context.Context, in RegisterInput) (LoginOutput, error) {
	// 先做用户名唯一性校验，避免无效事务开销。
	count, err := s.users.CountByUsername(ctx, in.Username)
	if err != nil {
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "检查用户名失败", err)
	}
	if count > 0 {
		return LoginOutput{}, newAppError(http.StatusBadRequest, "用户名已存在", nil)
	}

	hashedPassword, err := utils.HashPassword(in.Password)
	if err != nil {
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "密码加密失败", err)
	}

	user := models.User{
		Username:     in.Username,
		Password:     hashedPassword,
		Nickname:     in.Nickname,
		StorageQuota: config.AppConfig.Storage.DefaultUserQuota,
	}

	// 创建用户与根目录必须放在同一事务中，避免出现“有用户无根目录”的中间态。
	err = s.txManager.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := s.users.Create(ctx, tx, &user); err != nil {
			return err
		}
		_, err := s.resolver.getOrCreateUserRootFolder(ctx, tx, user.ID)
		return err
	})
	if err != nil {
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "创建用户失败", err)
	}

	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "生成令牌失败", err)
	}

	return LoginOutput{
		Token: token,
		User:  AuthUser{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
	}, nil
}

// Login 校验账号密码并签发访问令牌。
func (s *authService) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	user, err := s.users.GetByUsername(ctx, nil, in.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return LoginOutput{}, newAppError(http.StatusUnauthorized, "用户名或密码错误", nil)
		}
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "查询用户失败", err)
	}

	// 账号存在时仍需校验哈希密码，避免明文比较。
	if !utils.CheckPassword(in.Password, user.Password) {
		return LoginOutput{}, newAppError(http.StatusUnauthorized, "用户名或密码错误", nil)
	}

	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		return LoginOutput{}, newAppError(http.StatusInternalServerError, "生成令牌失败", err)
	}

	return LoginOutput{
		Token: token,
		User:  AuthUser{ID: user.ID, Username: user.Username, Nickname: user.Nickname},
	}, nil
}

// GetProfile 查询用户资料并确保根目录可用。
func (s *authService) GetProfile(ctx context.Context, userID uint) (ProfileOutput, error) {
	user, err := s.users.GetByID(ctx, nil, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ProfileOutput{}, newAppError(http.StatusNotFound, "用户不存在", nil)
		}
		return ProfileOutput{}, newAppError(http.StatusInternalServerError, "failed to query user", err)
	}

	// 对历史数据做兜底：若根目录缺失则自动补建。
	rootFolder, err := s.resolver.getOrCreateUserRootFolder(ctx, nil, user.ID)
	if err != nil {
		return ProfileOutput{}, newAppError(http.StatusInternalServerError, "加载根文件夹失败", err)
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
