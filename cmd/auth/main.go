package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	authserver "github.com/manzanit0/weathry/cmd/auth/proto/gen"
)

type server struct {
	authserver.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, in *authserver.HelloRequest) (*authserver.HelloReply, error) {
	return &authserver.HelloReply{Message: in.Name + " world"}, nil
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
	authserver.RegisterGreeterServer(s, &server{})
	// Serve gRPC Server
	log.Println("Serving gRPC on 0.0.0.0:8080")
	log.Fatal(s.Serve(lis))
}
