protoc:
	@protoc -I ./pkg/users \
   --go_out ./pkg/users/gen --go_opt paths=source_relative \
   --go-grpc_out ./pkg/users/gen --go-grpc_opt paths=source_relative \
   ./pkg/users/server.proto
