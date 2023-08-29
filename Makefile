# railway.app specific targets
rw-migrate:
	docker run --rm -v `pwd`/migrations:/flyway/sql flyway/flyway:7.14.0 \
	-url=jdbc:postgresql://`railway variables -s weathry --json | jq --raw-output .PGHOST`:`railway variables -s weathry --json | jq --raw-output .PGPORT`/`railway variables -s weathry --json | jq --raw-output .PGDATABASE` \
	-user=`railway variables -s weathry --json | jq --raw-output .PGUSER` \
	-password=`railway variables -s weathry --json | jq --raw-output .PGPASSWORD` \
	-schemas=public \
	-connectRetries=60 \
	migrate

rw-pgcli:
	pgcli `railway variables -s weathry --json | jq --raw-output .DATABASE_URL`
