package service

import (
	"context"
	"errors"
	"fmt"
	"go_user_service/config"
	"go_user_service/genproto/user_service"
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

type UserService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	redis    storage.IRedisStorage
}

func NewUserService(cfg config.Config, log logger.LoggerI, strg storage.StorageI, srvs client.ServiceManagerI, redis storage.IRedisStorage) *UserService {
	return &UserService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: srvs,
		redis:    redis,
	}
}

func (f *UserService) Create(ctx context.Context, req *user_service.CreateUser) (*user_service.GetUser, error) {

	f.log.Info("---CreateUser--->>>", logger.Any("req", req))

	resp, err := f.strg.User().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateUser--->>>", logger.Error(err))
		return &user_service.GetUser{}, err
	}

	return resp, nil
}
func (f *UserService) Update(ctx context.Context, req *user_service.UpdateUser) (*user_service.GetUser, error) {

	f.log.Info("---UpdateUser--->>>", logger.Any("req", req))

	resp, err := f.strg.User().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateUser--->>>", logger.Error(err))
		return &user_service.GetUser{}, err
	}

	return resp, nil
}

func (f *UserService) GetList(ctx context.Context, req *user_service.GetListUserRequest) (*user_service.GetListUserResponse, error) {
	f.log.Info("---GetListUser--->>>", logger.Any("req", req))

	resp, err := f.strg.User().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetListUser--->>>", logger.Error(err))
		return &user_service.GetListUserResponse{}, err
	}

	return resp, nil
}

func (f *UserService) GetByID(ctx context.Context, id *user_service.UserPrimaryKey) (*user_service.GetUser, error) {
	f.log.Info("---GetUser--->>>", logger.Any("req", id))

	resp, err := f.strg.User().GetById(ctx, id)
	if err != nil {
		f.log.Error("---GetUser--->>>", logger.Error(err))
		return &user_service.GetUser{}, err
	}

	return resp, nil
}

func (f *UserService) Delete(ctx context.Context, req *user_service.UserPrimaryKey) (*emptypb.Empty, error) {

	f.log.Info("---DeleteUser--->>>", logger.Any("req", req))

	_, err := f.strg.User().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteUser--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *UserService) Check(ctx context.Context, id *user_service.UserPrimaryKey) (*user_service.CheckUserResp, error) {
	f.log.Info("---GetUser--->>>", logger.Any("req", id))

	resp, err := f.strg.User().Check(ctx, id)
	if err != nil {
		f.log.Error("---GetUser--->>>", logger.Error(err))
		return &user_service.CheckUserResp{}, err
	}

	return resp, nil
}

func (a *UserService) Login(ctx context.Context, loginRequest *user_service.UserLoginRequest) (*user_service.UserLoginResponse, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.UserLogin)
	user, err := a.strg.User().GetByLogin(ctx, loginRequest.UserLogin)
	if err != nil {
		a.log.Error("error while getting user credentials by login", logger.Error(err))
		return &user_service.UserLoginResponse{}, err
	}

	if err = hash.CompareHashAndPassword(user.UserPassword, loginRequest.UserPassword); err != nil {
		a.log.Error("error while comparing password", logger.Error(err))
		return &user_service.UserLoginResponse{}, err
	}

	m := make(map[interface{}]interface{})

	m["user_id"] = user.Id
	m["user_role"] = config.STUDENT_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for user login", logger.Error(err))
		return &user_service.UserLoginResponse{}, err
	}

	return &user_service.UserLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *UserService) Register(ctx context.Context, loginRequest *user_service.UserRegisterRequest) (*emptypb.Empty, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.Mail)

	otpCode := pkg.GenerateOTP()

	msg := fmt.Sprintf("Your otp code is: %v, for registering CRM system. Don't give it to anyone", otpCode)

	err := a.redis.SetX(ctx, loginRequest.Mail, otpCode, time.Minute*2)
	if err != nil {
		a.log.Error("error while setting otpCode to redis user register", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	err = smtp.SendMail(loginRequest.Mail, msg)
	if err != nil {
		a.log.Error("error while sending otp code to user register", logger.Error(err))
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UserService) RegisterConfirm(ctx context.Context, req *user_service.UserRegisterConfRequest) (*user_service.UserLoginResponse, error) {
	resp := &user_service.UserLoginResponse{}

	otp, err := a.redis.Get(ctx, req.Mail)
	if err != nil {
		a.log.Error("error while getting otp code for user register confirm", logger.Error(err))
		return resp, err
	}
	if req.Otp != otp {
		a.log.Error("incorrect otp code for user register confirm", logger.Error(err))
		return resp, errors.New("incorrect otp code")
	}
	req.User[0].Email = req.Mail

	id, err := a.strg.User().Create(ctx, req.User[0])
	if err != nil {
		a.log.Error("error while creating user", logger.Error(err))
		return resp, err
	}
	var m = make(map[interface{}]interface{})

	m["user_id"] = id
	m["user_role"] = config.STUDENT_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for user register confirm", logger.Error(err))
		return resp, err
	}
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken

	return resp, nil
}

func (f *UserService) ChangePassword(ctx context.Context, pass *user_service.UserChangePassword) (*user_service.UserChangePasswordResp, error) {
	f.log.Info("---ChangePassword--->>>", logger.Any("req", pass))

	resp, err := f.strg.User().ChangePassword(ctx, pass)
	if err != nil {
		f.log.Error("---ChangePassword--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}
