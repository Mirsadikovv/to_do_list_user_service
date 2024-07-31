package service

import (
	"context"
	"errors"
	"fmt"
	"go_user_service/config"
	"go_user_service/genproto/administrator_service"
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

type AdministratorService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	redis    storage.IRedisStorage
}

func NewAdministratorService(cfg config.Config, log logger.LoggerI, strg storage.StorageI, srvs client.ServiceManagerI, redis storage.IRedisStorage) *AdministratorService {
	return &AdministratorService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: srvs,
		redis:    redis,
	}
}

func (f *AdministratorService) Create(ctx context.Context, req *administrator_service.CreateAdministrator) (*administrator_service.GetAdministrator, error) {

	f.log.Info("---CreateAdministrator--->>>", logger.Any("req", req))

	resp, err := f.strg.Administrator().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateAdministrator--->>>", logger.Error(err))
		return &administrator_service.GetAdministrator{}, err
	}

	return resp, nil
}
func (f *AdministratorService) Update(ctx context.Context, req *administrator_service.UpdateAdministrator) (*administrator_service.GetAdministrator, error) {

	f.log.Info("---UpdateAdministrator--->>>", logger.Any("req", req))

	resp, err := f.strg.Administrator().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateAdministrator--->>>", logger.Error(err))
		return &administrator_service.GetAdministrator{}, err
	}

	return resp, nil
}

func (f *AdministratorService) GetList(ctx context.Context, req *administrator_service.GetListAdministratorRequest) (*administrator_service.GetListAdministratorResponse, error) {
	f.log.Info("---GetListAdministrator--->>>", logger.Any("req", req))

	resp, err := f.strg.Administrator().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetListAdministrator--->>>", logger.Error(err))
		return &administrator_service.GetListAdministratorResponse{}, err
	}

	return resp, nil
}

func (f *AdministratorService) GetByID(ctx context.Context, id *administrator_service.AdministratorPrimaryKey) (*administrator_service.GetAdministrator, error) {
	f.log.Info("---GetAdministrator--->>>", logger.Any("req", id))

	resp, err := f.strg.Administrator().GetById(ctx, id)
	if err != nil {
		f.log.Error("---GetAdministrator--->>>", logger.Error(err))
		return &administrator_service.GetAdministrator{}, err
	}

	return resp, nil
}

func (f *AdministratorService) Delete(ctx context.Context, req *administrator_service.AdministratorPrimaryKey) (*emptypb.Empty, error) {

	f.log.Info("---DeleteAdministrator--->>>", logger.Any("req", req))

	_, err := f.strg.Administrator().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteAdministrator--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (a *AdministratorService) Login(ctx context.Context, loginRequest *administrator_service.AdministratorLoginRequest) (*administrator_service.AdministratorLoginResponse, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.UserLogin)
	administrator, err := a.strg.Administrator().GetByLogin(ctx, loginRequest.UserLogin)
	if err != nil {
		a.log.Error("error while getting administrator credentials by login", logger.Error(err))
		return &administrator_service.AdministratorLoginResponse{}, err
	}

	if err = hash.CompareHashAndPassword(administrator.UserPassword, loginRequest.UserPassword); err != nil {
		a.log.Error("error while comparing password", logger.Error(err))
		return &administrator_service.AdministratorLoginResponse{}, err
	}

	m := make(map[interface{}]interface{})

	m["user_id"] = administrator.Id
	m["user_role"] = config.ADMINISTRATOR_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for administrator login", logger.Error(err))
		return &administrator_service.AdministratorLoginResponse{}, err
	}

	return &administrator_service.AdministratorLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *AdministratorService) Register(ctx context.Context, loginRequest *administrator_service.AdministratorRegisterRequest) (*emptypb.Empty, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.Mail)

	otpCode := pkg.GenerateOTP()

	msg := fmt.Sprintf("Your otp code is: %v, for registering CRM system. Don't give it to anyone", otpCode)

	err := a.redis.SetX(ctx, loginRequest.Mail, otpCode, time.Minute*2)
	if err != nil {
		a.log.Error("error while setting otpCode to redis administrator register", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	err = smtp.SendMail(loginRequest.Mail, msg)
	if err != nil {
		a.log.Error("error while sending otp code to administrator register", logger.Error(err))
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *AdministratorService) RegisterConfirm(ctx context.Context, req *administrator_service.AdministratorRegisterConfRequest) (*administrator_service.AdministratorLoginResponse, error) {
	resp := &administrator_service.AdministratorLoginResponse{}

	otp, err := a.redis.Get(ctx, req.Mail)
	if err != nil {
		a.log.Error("error while getting otp code for administrator register confirm", logger.Error(err))
		return resp, err
	}
	if req.Otp != otp {
		a.log.Error("incorrect otp code for administrator register confirm", logger.Error(err))
		return resp, errors.New("incorrect otp code")
	}
	req.Administrator[0].Email = req.Mail

	id, err := a.strg.Administrator().Create(ctx, req.Administrator[0])
	if err != nil {
		a.log.Error("error while creating administrator", logger.Error(err))
		return resp, err
	}
	var m = make(map[interface{}]interface{})

	m["user_id"] = id
	m["user_role"] = config.ADMINISTRATOR_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for administrator register confirm", logger.Error(err))
		return resp, err
	}
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken

	return resp, nil
}

func (f *AdministratorService) ChangePassword(ctx context.Context, pass *administrator_service.AdministratorChangePassword) (*administrator_service.AdministratorChangePasswordResp, error) {
	f.log.Info("---ChangePassword--->>>", logger.Any("req", pass))

	resp, err := f.strg.Administrator().ChangePassword(ctx, pass)
	if err != nil {
		f.log.Error("---ChangePassword--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}
