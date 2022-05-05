#!/bin/bash

arrService=(server api backend web)
for i in "${arrService[@]}"; do
    if [ "$i" == "$1" ]; then
        service_name=$i
        break
    fi
done
if [ -z "$service_name" ]; then
    echo "Service \"$1\" not found"
    exit 0
fi

document_root=/data/video-call
service_path=$service_name
service=$document_root/$service_path/service
num_prog=1
config=""

if [ "$service_name" == "server" ]; then
	service_path="server/bin"
	service=$document_root/$service_path/livekit-server
	config="--config=/data/video-call/server/config.yaml"
elif [ "$service_name" == "web" ]; then
	service_path="frontend/web"
	service=$document_root/$service_path/service
fi

log_file=$document_root/logs/$service_name/service-$(date +"%Y-%m-%d").log

start() {
    cd $document_root/$service_path || return
	echo "Starting $service"
    nohup "$service" "$config" >> "$log_file" 2>&1 &
}

stop() {
	echo "Stopping $service"
	pid=$(pgrep -f "$service")
    if [ -n "$pid" ]; then
        kill -9 "$pid"
    fi
}

detect() {
	current_num_prog=$(pgrep -fc "$service")
	if [ "$current_num_prog" -lt "$num_prog" ]; then
		(( new_prog="$num_prog"-"$current_num_prog" ))
		i=1
		while [ $i -le $new_prog ]; do
			start
			(( i++ ))
		done
	fi
}

graceful() {
	pid=$(pgrep -f "$service")
    if [ -n "$pid" ]; then
        kill "$pid"
    fi
}

case "$2" in
	"start" )
        start
    	;;
	"stop" )
		stop
        ;;
	"restart" )
		stop
		detect
    	;;
	"graceful" )
		graceful
		detect
		;;
	"detect" )
    	detect
       ;;
    * )
    	echo "Usage: {start|stop|restart|detect|graceful)"
        exit 1
esac
usleep 500
pgrep -fa "$service"
exit 0
