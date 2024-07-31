#!/bin/bash

# Получаем текущий каталог из первого аргумента
CURRENT_DIR=$1

# Удаляем старые сгенерированные файлы
rm -rf "${CURRENT_DIR}/genproto/*"

# Устанавливаем пути к плагинам
PROTOC_GEN_GO=$(which protoc-gen-go)
PROTOC_GEN_GO_GRPC=$(which protoc-gen-go-grpc)

# Проверяем, что плагины найдены
if [[ -z "$PROTOC_GEN_GO" || -z "$PROTOC_GEN_GO_GRPC" ]]; then
  echo "protoc-gen-go or protoc-gen-go-grpc not found in PATH"
  exit 1
fi

# Генерация кода для каждого подкаталога в папке protos
for x in $(find ${CURRENT_DIR}/crm_proto/* -type d); do
  protoc --plugin="protoc-gen-go=${PROTOC_GEN_GO}" \
         --plugin="protoc-gen-go-grpc=${PROTOC_GEN_GO_GRPC}" \
         -I=${x} -I=${CURRENT_DIR}/crm_proto -I /usr/local/include \
         --go_out=${CURRENT_DIR} \
         --go-grpc_out=require_unimplemented_servers=false:${CURRENT_DIR} \
         ${x}/*.proto
done







