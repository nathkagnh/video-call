#!/bin/bash

Run() {
    cd /data/video-call/scripts || return;
    
    /bin/sh service.sh "server" "$1"
    /bin/sh service.sh "api" "$1"
    /bin/sh service.sh "backend" "$1"
    /bin/sh service.sh "web" "$1"
}

case "$1" in
    "start" )
        Run start
        ;;
    "stop" )
        Run stop
        ;;
    "restart" )
        Run restart
        ;;
    "graceful" )
        Run graceful
        ;;
    "detect" )
        Run detect
        ;;
    * )
        echo "Usage: {start|stop|restart|detect|graceful)"
        exit 1
esac
exit 0