CURRENT_DIR=$(shell pwd)

APP=$(shell basename ${CURRENT_DIR})
APP_CMD_DIR=${CURRENT_DIR}/cmd

gen-proto-module:
	sudo rm -rf ${CURRENT_DIR}/genproto
	./scripts/gen_proto.sh ${CURRENT_DIR}

migrateup:
	migrate -path ./migrations -database 'postgres://mirodil:1212@localhost:5432/to_do_list_user?sslmode=disable' up

migratedown:
	migrate -path ./migrations -database 'postgres://mirodil:1212@localhost:5432/to_do_list_user?sslmode=disable' down