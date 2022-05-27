package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authserver "github.com/manzanit0/weathry/cmd/users/proto/gen"
)

type server struct {
	authserver.UnimplementedUsersServer
	Users UsersRepository
}

func (s *server) Create(ctx context.Context, in *authserver.CreateRequest) (*authserver.CreateResponse, error) {
	var u User
	u.TelegramChatID = in.GetTelegramChatId()
	u.FirstName = in.GetFirstName()
	u.LastName = in.GetLastName()
	u.Username = in.GetUsername()
	u.LanguageCode = in.GetLanguageCode()

	_, err := s.Users.Create(u)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to create user: %s", err.Error()))
	}

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

type User struct {
	TelegramChatID int64
	Username       string
	FirstName      string
	LastName       string
	LanguageCode   string
}

type UsersRepository struct{}

func (r *UsersRepository) Create(u User) (User, error) {
	return u, nil
}
