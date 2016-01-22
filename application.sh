#!/bin/bash

# 切换到当前目录
dirname $0
cd `dirname $0`

# 设置路径
CUR_PATH=`pwd`

export GOPATH=$GOPATH:$CUR_PATH

# 显示需要的环境变量
echo APP_HOST=$APP_HOST
echo APP_PORT=$APP_PORT

echo DB_HOST=$DB_HOST
echo DB_PORT=$DB_PORT
echo DB_NAME=$DB_NAME
echo DB_USER=$DB_USER
echo DB_PASS=$DB_PASS

echo REDIS_HOST=$REDIS_HOST
echo REDIS_PORT=$REDIS_PORT

echo GOPATH=$GOPATH

# 编译工程
cd ./application
go build

# 运行程序
cmd="${1}"
case ${cmd} in
   -r) ./application
      ;;
   -t) ./application
      ;;
    *)
      echo "`basename ${0}`:usage: [-r run] | [-t test]"
      ;;
esac