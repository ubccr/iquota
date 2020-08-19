#!/bin/bash


CLIENT_DIR='./.client-release'
VERSION=`grep Version cmd/iquota/main.go | egrep -o '[0-9]\.[0-9]\.[0-9]'`
NAME=iquota-${VERSION}-linux-amd64
REL_DIR=${CLIENT_DIR}/${NAME}

rm -Rf ${CLIENT_DIR}
mkdir -p ${REL_DIR}

cp ./cmd/iquota/iquota ${REL_DIR}/ 
cp ./cmd/iquota/iquota.yaml.sample ${REL_DIR}/ 
cp ./cmd/iquota/iquota.spec ${REL_DIR}/
cp ./README.rst ${REL_DIR}/ 
cp ./AUTHORS.rst ${REL_DIR}/ 
cp ./ChangeLog.rst ${REL_DIR}/ 
cp ./LICENSE ${REL_DIR}/ 

tar -C ${CLIENT_DIR} -cvzf ${NAME}.tar.gz ${NAME}
rm -Rf ${CLIENT_DIR}

SERVER_DIR='./.server-release'
VERSION=`grep Version cmd/iquota-server/main.go | egrep -o '[0-9]\.[0-9]\.[0-9]'`
NAME=iquota-server-${VERSION}-linux-amd64
REL_DIR=${SERVER_DIR}/${NAME}

rm -Rf ${SERVER_DIR}
mkdir -p ${REL_DIR}

cp ./cmd/iquota-server/iquota-server ${REL_DIR}/ 
cp ./cmd/ipanfs/ipanfs ${REL_DIR}/ 
cp ./cmd/iquota-server/iquota.yaml.sample ${REL_DIR}/ 
cp ./cmd/iquota-server/iquota-server.spec ${REL_DIR}/
cp ./README.rst ${REL_DIR}/ 
cp ./AUTHORS.rst ${REL_DIR}/ 
cp ./ChangeLog.rst ${REL_DIR}/ 
cp ./LICENSE ${REL_DIR}/ 

tar -C ${SERVER_DIR} -cvzf ${NAME}.tar.gz ${NAME}
rm -Rf ${SERVER_DIR}
