package service

import (
	"context"
	"errors"
	"fmt"
	"go_user_service/config"
	"go_user_service/genproto/student_service"
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

type StudentService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	redis    storage.IRedisStorage
}

func NewStudentService(cfg config.Config, log logger.LoggerI, strg storage.StorageI, srvs client.ServiceManagerI, redis storage.IRedisStorage) *StudentService {
	return &StudentService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: srvs,
		redis:    redis,
	}
}

func (f *StudentService) Create(ctx context.Context, req *student_service.CreateStudent) (*student_service.GetStudent, error) {

	f.log.Info("---CreateStudent--->>>", logger.Any("req", req))

	resp, err := f.strg.Student().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateStudent--->>>", logger.Error(err))
		return &student_service.GetStudent{}, err
	}

	return resp, nil
}
func (f *StudentService) Update(ctx context.Context, req *student_service.UpdateStudent) (*student_service.GetStudent, error) {

	f.log.Info("---UpdateStudent--->>>", logger.Any("req", req))

	resp, err := f.strg.Student().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateStudent--->>>", logger.Error(err))
		return &student_service.GetStudent{}, err
	}

	return resp, nil
}

func (f *StudentService) GetList(ctx context.Context, req *student_service.GetListStudentRequest) (*student_service.GetListStudentResponse, error) {
	f.log.Info("---GetListStudent--->>>", logger.Any("req", req))

	resp, err := f.strg.Student().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetListStudent--->>>", logger.Error(err))
		return &student_service.GetListStudentResponse{}, err
	}

	return resp, nil
}

func (f *StudentService) GetByID(ctx context.Context, id *student_service.StudentPrimaryKey) (*student_service.GetStudent, error) {
	f.log.Info("---GetStudent--->>>", logger.Any("req", id))

	resp, err := f.strg.Student().GetById(ctx, id)
	if err != nil {
		f.log.Error("---GetStudent--->>>", logger.Error(err))
		return &student_service.GetStudent{}, err
	}

	return resp, nil
}

func (f *StudentService) Delete(ctx context.Context, req *student_service.StudentPrimaryKey) (*emptypb.Empty, error) {

	f.log.Info("---DeleteStudent--->>>", logger.Any("req", req))

	_, err := f.strg.Student().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteStudent--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *StudentService) Check(ctx context.Context, id *student_service.StudentPrimaryKey) (*student_service.CheckStudentResp, error) {
	f.log.Info("---GetStudent--->>>", logger.Any("req", id))

	resp, err := f.strg.Student().Check(ctx, id)
	if err != nil {
		f.log.Error("---GetStudent--->>>", logger.Error(err))
		return &student_service.CheckStudentResp{}, err
	}

	return resp, nil
}

func (a *StudentService) Login(ctx context.Context, loginRequest *student_service.StudentLoginRequest) (*student_service.StudentLoginResponse, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.UserLogin)
	student, err := a.strg.Student().GetByLogin(ctx, loginRequest.UserLogin)
	if err != nil {
		a.log.Error("error while getting student credentials by login", logger.Error(err))
		return &student_service.StudentLoginResponse{}, err
	}

	if err = hash.CompareHashAndPassword(student.UserPassword, loginRequest.UserPassword); err != nil {
		a.log.Error("error while comparing password", logger.Error(err))
		return &student_service.StudentLoginResponse{}, err
	}

	m := make(map[interface{}]interface{})

	m["user_id"] = student.Id
	m["user_role"] = config.STUDENT_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for student login", logger.Error(err))
		return &student_service.StudentLoginResponse{}, err
	}

	return &student_service.StudentLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *StudentService) Register(ctx context.Context, loginRequest *student_service.StudentRegisterRequest) (*emptypb.Empty, error) {
	fmt.Println(" loginRequest.Login: ", loginRequest.Mail)

	otpCode := pkg.GenerateOTP()

	msg := fmt.Sprintf("Your otp code is: %v, for registering CRM system. Don't give it to anyone", otpCode)

	err := a.redis.SetX(ctx, loginRequest.Mail, otpCode, time.Minute*2)
	if err != nil {
		a.log.Error("error while setting otpCode to redis student register", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	err = smtp.SendMail(loginRequest.Mail, msg)
	if err != nil {
		a.log.Error("error while sending otp code to student register", logger.Error(err))
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *StudentService) RegisterConfirm(ctx context.Context, req *student_service.StudentRegisterConfRequest) (*student_service.StudentLoginResponse, error) {
	resp := &student_service.StudentLoginResponse{}

	otp, err := a.redis.Get(ctx, req.Mail)
	if err != nil {
		a.log.Error("error while getting otp code for student register confirm", logger.Error(err))
		return resp, err
	}
	if req.Otp != otp {
		a.log.Error("incorrect otp code for student register confirm", logger.Error(err))
		return resp, errors.New("incorrect otp code")
	}
	req.Student[0].Email = req.Mail

	id, err := a.strg.Student().Create(ctx, req.Student[0])
	if err != nil {
		a.log.Error("error while creating student", logger.Error(err))
		return resp, err
	}
	var m = make(map[interface{}]interface{})

	m["user_id"] = id
	m["user_role"] = config.STUDENT_ROLE

	accessToken, refreshToken, err := jwt.GenJWT(m)
	if err != nil {
		a.log.Error("error while generating tokens for student register confirm", logger.Error(err))
		return resp, err
	}
	resp.AccessToken = accessToken
	resp.RefreshToken = refreshToken

	return resp, nil
}

func (f *StudentService) ChangePassword(ctx context.Context, pass *student_service.StudentChangePassword) (*student_service.StudentChangePasswordResp, error) {
	f.log.Info("---ChangePassword--->>>", logger.Any("req", pass))

	resp, err := f.strg.Student().ChangePassword(ctx, pass)
	if err != nil {
		f.log.Error("---ChangePassword--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}
