package grpc

import (
	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"net"
)

type App struct {
	proto.UnimplementedDirectoryServiceServer

	srv *grpc.Server
}

func NewGrpc() *App {
	server := &App{
		srv: grpc.NewServer(),
	}

	health.RegisterHealthServer(server.srv, server)
	proto.RegisterDirectoryServiceServer(server.srv, server)

	reflection.Register(server.srv)

	return server
}

func (v *App) Listen() error {
	listener, err := net.Listen("tcp", viper.GetString("grpc_bind"))
	if err != nil {
		return err
	}

	return v.srv.Serve(listener)
}
