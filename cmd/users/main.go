package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	_ "github.com/jackc/pgx/v4/stdlib"
	authserver "github.com/manzanit0/weathry/pkg/users/gen"
)

type server struct {
	authserver.UnimplementedUsersServer
	Users UsersRepository
}

func (s *server) Create(ctx context.Context, in *authserver.CreateRequest) (*authserver.CreateResponse, error) {
	log.Println("Received grpc.UsersServer/Create")
	u, err := s.Users.Find(ctx, fmt.Sprint(in.GetTelegramChatId()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to query for user: %s", err.Error()))
	}

	if errors.Is(err, sql.ErrNoRows) {
		u = &User{}
		u.TelegramChatID = in.GetTelegramChatId()
		u.FirstName = in.GetFirstName()
		u.LastName = in.GetLastName()
		u.Username = in.GetUsername()
		u.LanguageCode = in.GetLanguageCode()

		_, err := s.Users.Create(ctx, *u)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to create user: %s", err.Error()))
		}
	}

	return &authserver.CreateResponse{}, nil
}

func main() {
	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	db, err := sql.Open("pgx", fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", os.Getenv("PGUSER"), os.Getenv("PGPASSWORD"), os.Getenv("PGHOST"), os.Getenv("PGPORT"), os.Getenv("PGDATABASE")))
	if err != nil {
		panic(fmt.Errorf("unable to open db conn: %w", err))
	}

	defer func() {
		err = db.Close()
		if err != nil {
			log.Println("error closing db connection: %w", err)
		}
	}()

	// Create a gRPC server object
	s := grpc.NewServer()
	// Attach the Greeter service to the server
	authserver.RegisterUsersServer(s, &server{Users: UsersRepository{db}})
	// Serve gRPC Server
	log.Println("Serving gRPC on ", lis.Addr().String())
	log.Fatal(s.Serve(lis)) // TODO: handle graceful shutdowns.
}
