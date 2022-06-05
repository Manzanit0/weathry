protoc:
	@protoc -I ./pkg/users \
   --go_out ./pkg/users/gen --go_opt paths=source_relative \
   --go-grpc_out ./pkg/users/gen --go-grpc_opt paths=source_relative \
   ./pkg/users/server.proto

# railway.app specific targets
rw-migrate:
	docker run --rm -v `pwd`/migrations:/flyway/sql flyway/flyway:7.14.0 \
	-url=jdbc:postgresql://`railway variables get PGHOST -s weathry`:`railway variables get PGPORT -s weathry`/`railway variables get PGDATABASE -s weathry` \
	-user=`railway variables get PGUSER -s weathry` \
	-password=`railway variables get PGPASSWORD -s weathry` \
	-schemas=public \
	-connectRetries=60 \
	migrate

rw-pgcli:
	pgcli `railway variables get DATABASE_URL -s weathry`
