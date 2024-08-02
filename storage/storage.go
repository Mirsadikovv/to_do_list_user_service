package storage

import (
	"context"
	"go_user_service/genproto/admin_service"
	"go_user_service/genproto/user_service"

	"time"

	"google.golang.org/protobuf/types/known/emptypb"
)

type StorageI interface {
	CloseDB()
	Admin() AdminRepoI
	User() UserRepoI
	Redis() IRedisStorage
}

type AdminRepoI interface {
	Create(context.Context, *admin_service.CreateAdmin) (*admin_service.GetAdmin, error)
	Update(context.Context, *admin_service.UpdateAdmin) (*admin_service.GetAdmin, error)
	GetAll(context.Context, *admin_service.GetListAdminRequest) (*admin_service.GetListAdminResponse, error)
	GetById(context.Context, *admin_service.AdminPrimaryKey) (*admin_service.GetAdmin, error)
	Delete(context.Context, *admin_service.AdminPrimaryKey) (emptypb.Empty, error)
	ChangePassword(context.Context, *admin_service.AdminChangePassword) (*admin_service.AdminChangePasswordResp, error)
	GetByLogin(context.Context, string) (*admin_service.GetAdminByLogin, error)
	GetPassword(context.Context, string) (string, error)
}

type UserRepoI interface {
	Create(context.Context, *user_service.CreateUser) (*user_service.GetUser, error)
	Update(context.Context, *user_service.UpdateUser) (*user_service.GetUser, error)
	GetAll(context.Context, *user_service.GetListUserRequest) (*user_service.GetListUserResponse, error)
	GetById(context.Context, *user_service.UserPrimaryKey) (*user_service.GetUser, error)
	Delete(context.Context, *user_service.UserPrimaryKey) (emptypb.Empty, error)
	Check(context.Context, *user_service.UserPrimaryKey) (*user_service.CheckUserResp, error)
	ChangePassword(context.Context, *user_service.UserChangePassword) (*user_service.UserChangePasswordResp, error)
	GetByLogin(context.Context, string) (*user_service.GetUserByLogin, error)
	GetPassword(context.Context, string) (string, error)
}

type IRedisStorage interface {
	SetX(context.Context, string, interface{}, time.Duration) error
	Get(context.Context, string) (interface{}, error)
	Del(context.Context, string) error
}
