package storage

import (
	"context"
	"go_user_service/genproto/administrator_service"
	"go_user_service/genproto/student_service"

	"time"

	"google.golang.org/protobuf/types/known/emptypb"
)

type StorageI interface {
	CloseDB()
	Administrator() AdministratorRepoI
	Student() StudentRepoI
	Redis() IRedisStorage
}

type AdministratorRepoI interface {
	Create(context.Context, *administrator_service.CreateAdministrator) (*administrator_service.GetAdministrator, error)
	Update(context.Context, *administrator_service.UpdateAdministrator) (*administrator_service.GetAdministrator, error)
	GetAll(context.Context, *administrator_service.GetListAdministratorRequest) (*administrator_service.GetListAdministratorResponse, error)
	GetById(context.Context, *administrator_service.AdministratorPrimaryKey) (*administrator_service.GetAdministrator, error)
	Delete(context.Context, *administrator_service.AdministratorPrimaryKey) (emptypb.Empty, error)
	ChangePassword(context.Context, *administrator_service.AdministratorChangePassword) (*administrator_service.AdministratorChangePasswordResp, error)
	GetByLogin(context.Context, string) (*administrator_service.GetAdministratorByLogin, error)
	GetPassword(context.Context, string) (string, error)
}

type StudentRepoI interface {
	Create(context.Context, *student_service.CreateStudent) (*student_service.GetStudent, error)
	Update(context.Context, *student_service.UpdateStudent) (*student_service.GetStudent, error)
	GetAll(context.Context, *student_service.GetListStudentRequest) (*student_service.GetListStudentResponse, error)
	GetById(context.Context, *student_service.StudentPrimaryKey) (*student_service.GetStudent, error)
	Delete(context.Context, *student_service.StudentPrimaryKey) (emptypb.Empty, error)
	Check(context.Context, *student_service.StudentPrimaryKey) (*student_service.CheckStudentResp, error)
	ChangePassword(context.Context, *student_service.StudentChangePassword) (*student_service.StudentChangePasswordResp, error)
	GetByLogin(context.Context, string) (*student_service.GetStudentByLogin, error)
	GetPassword(context.Context, string) (string, error)
}

type IRedisStorage interface {
	SetX(context.Context, string, interface{}, time.Duration) error
	Get(context.Context, string) (interface{}, error)
	Del(context.Context, string) error
}
