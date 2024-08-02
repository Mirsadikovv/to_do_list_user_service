package grpc

import (
	"go_user_service/config"
	"go_user_service/genproto/admin_service"
	"go_user_service/genproto/user_service"

	"go_user_service/grpc/client"
	"go_user_service/grpc/service"
	"go_user_service/storage"

	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func SetUpServer(cfg config.Config, log logger.LoggerI, strg storage.StorageI, srvc client.ServiceManagerI, redis storage.IRedisStorage) (grpcServer *grpc.Server) {

	grpcServer = grpc.NewServer()

	admin_service.RegisterAdminServiceServer(grpcServer, service.NewAdminService(cfg, log, strg, srvc, redis))
	user_service.RegisterUserServiceServer(grpcServer, service.NewUserService(cfg, log, strg, srvc, redis))

	reflection.Register(grpcServer)
	return
}
