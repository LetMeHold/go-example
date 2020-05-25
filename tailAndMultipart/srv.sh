#!/bin/bash

cd `dirname $0`
DEPLOY_DIR=`pwd`
SERVER_NAME=NginxLogTail
EXE_FILE=$DEPLOY_DIR/tlog
LOG_DIR=$DEPLOY_DIR

function start()
{
    PIDS=`ps -ef | grep "$EXE_FILE" | grep -v grep | awk '{print $2}'`
    if [ -n "$PIDS" ]; then
        echo "ERROR: The $SERVER_NAME already started!"
        echo "PID: $PIDS"
        exit 1
    fi
    nohup $EXE_FILE >> $LOG_DIR/std.out 2>&1 &
}

function stop()
{
    PIDS=`ps -ef | grep "$EXE_FILE" | grep -v grep | awk '{print $2}'`
    if [ -z "$PIDS" ]; then
        echo "ERROR: The $SERVER_NAME not started!"
    else
        kill $PIDS
    fi
}

if [ "$1" == "start" ];then
    start
elif [ "$1" == "stop" ];then
    stop
elif [ "$1" == "restart" ];then
    stop
    sleep 3
    start
else
    echo "srv.sh [start/stop/restart]"
fi
