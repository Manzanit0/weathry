package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	authserver "github.com/manzanit0/weathry/cmd/users/proto/gen"
)

type server struct {
	authserver.UnimplementedUsersServer
}

func (s *server) SayHello(ctx context.Context, in *authserver.CreateRequest) (*authserver.CreateResponse, error) {
	return &authserver.CreateResponse{}, nil
}

func main() {
	// Create a listener on TCP port
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	// Create a gRPC server object
	s := grpc.NewServer()
	// Attach the Greeter service to the server
	authserver.RegisterUsersServer(s, &server{})
	// Serve gRPC Server
	log.Println("Serving gRPC on 0.0.0.0:8080")
	log.Fatal(s.Serve(lis))
}
