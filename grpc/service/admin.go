package service

import (
	"context"
	"errors"
	"fmt"
	"go_user_service/config"
	"go_user_service/genproto/admin_service"
	"go_user_service/pkg"
	"go_user_service/pkg/hash"
	"go_user_service/pkg/jwt"
	"go_user_service/pkg/smtp"
	"time"

	"go_user_service/grpc/client"
	"go_user_service/storage"

	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AdminService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	redis    storage.IRedisStorage
}

func NewAdminService(cfg config.Config, log logger.LoggerI, strg storage.StorageI, srvs client.ServiceManagerI, redis storage.IRedisStorage) *AdminService {
	return &AdminService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: srvs,
		redis:    redis,
	}
}

func (f *AdminService) Create(ctx context.Context, req *admin_service.CreateAdmin) (*admin_service.GetAdmin, error) {

	f.log.Info("---CreateAdmin--->>>", logger.Any("req", req))

	resp, err := f.strg.Admin().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateAdmin--->>>", logger.Error(err))
		return &admin_service.GetAdmin{}, err
	}

	return resp, nil
}
func (f *AdminService) Update(ctx context.Context, req *admin_service.UpdateAdmin) (*admin_service.GetAdmin, error) {

	f.log.Info("---UpdateAdmin--->>>", logger.Any("req", req))

	resp, err := f.strg.Admin().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateAdmin--->>>", logger.Error(err))
		return &admin_service.GetAdmin{}, err
	}

	return resp, nil
}

func (f *AdminService) GetList(ctx context.Context, req *admin_service.GetListAdminRequest) (*admin_service.GetListAdminResponse, error) {
	f.log.Info("---GetListAdmin--->>>", logger.Any("req", req))

	resp, err := f.strg.Admin().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetListAdmin--->>>", logger.Error(err))
		return &admin_service.GetListAdminResponse{}, err
	}

	return resp, nil
}

func (f *AdminService) GetByID(ctx context.Context, id *admin_service.AdminPrimaryKey) (*admin_service.GetAdmin, error) {
	f.log.Info("---GetAdmin--->>>", logger.Any("req", id))

	resp, err := f.strg.Admin().GetById(ctx, id)
	if err != nil {
		f.log.Error("---GetAdmin--->>>", logger.Error(err))
		return &admin_service.GetAdmin{}, err
	}

	return resp, nil
}

func (f *AdminService) Delete(ctx context.Context, req *admin_service.AdminPrimaryKey) (*emptypb.Empty, error) {

	f.log.Info("---DeleteAdmin--->>>", logger.Any("req", req))

	_, err := f.strg.Admin().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteAdmin--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (a *AdminService) Login(ctx context.Context, loginRequest *admin_service.AdminLoginRequest) (*admin_service.AdminLoginResponse, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.UserLogin)
	admin, err := a.strg.Admin().GetByLogin(ctx, loginRequest.UserLogin)
	if err != nil {
		a.log.Error("error while getting admin credentials by login", logger.Error(err))
		return &admin_service.AdminLoginResponse{}, err
	}

	if err = hash.CompareHashAndPassword(admin.UserPassword, loginRequest.UserPassword); err != nil {
		a.log.Error("error while comparing password", logger.Error(err))
		return &admin_service.AdminLoginResponse{}, err
	}

	m := make(map[interface{}]interface{})

	m["user_id"] = admin.Id
	m["user_role"] = config.ADMIN_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for admin login", logger.Error(err))
		return &admin_service.AdminLoginResponse{}, err
	}

	return &admin_service.AdminLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *AdminService) Register(ctx context.Context, loginRequest *admin_service.AdminRegisterRequest) (*emptypb.Empty, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.Mail)

	otpCode := pkg.GenerateOTP()
	fmt.Println(" loginRequest.Pass: ", otpCode)

	msg := fmt.Sprintf("Your otp code is: %v, for registering TODO_LIST. Don't give it to anyone", otpCode)

	err := a.redis.SetX(ctx, loginRequest.Mail, otpCode, time.Minute*2)
	if err != nil {
		a.log.Error("error while setting otpCode to redis admin register", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	err = smtp.SendMail(loginRequest.Mail, msg)
	if err != nil {
		a.log.Error("error while sending otp code to admin register", logger.Error(err))
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *AdminService) RegisterConfirm(ctx context.Context, req *admin_service.AdminRegisterConfRequest) (*admin_service.AdminLoginResponse, error) {
	resp := &admin_service.AdminLoginResponse{}

	otp, err := a.redis.Get(ctx, req.Mail)
	if err != nil {
		a.log.Error("error while getting otp code for admin register confirm", logger.Error(err))
		return resp, err
	}
	if req.Otp != otp {
		a.log.Error("incorrect otp code for admin register confirm", logger.Error(err))
		return resp, errors.New("incorrect otp code")
	}
	req.Admin[0].Email = req.Mail

	id, err := a.strg.Admin().Create(ctx, req.Admin[0])
	if err != nil {
		a.log.Error("error while creating admin", logger.Error(err))
		return resp, err
	}
	var m = make(map[interface{}]interface{})

	m["user_id"] = id
	m["user_role"] = config.ADMIN_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for admin register confirm", logger.Error(err))
		return resp, err
	}
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken

	return resp, nil
}

func (f *AdminService) ChangePassword(ctx context.Context, pass *admin_service.AdminChangePassword) (*admin_service.AdminChangePasswordResp, error) {
	f.log.Info("---ChangePassword--->>>", logger.Any("req", pass))

	resp, err := f.strg.Admin().ChangePassword(ctx, pass)
	if err != nil {
		f.log.Error("---ChangePassword--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}
