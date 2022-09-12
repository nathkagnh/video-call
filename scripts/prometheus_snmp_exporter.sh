#!/bin/bash
service_name=snmp_exporter
document_root=/data/video-call
service_path=prometheus/snmp_exporter-0.20.0.linux-amd64
service=$document_root/$service_path/$service_name
num_prog=1
config="--config.file=$document_root/$service_path/snmp.yml"

log_file=$document_root/logs/prometheus/snmp_exporter-$(date +"%Y-%m-%d").log

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

case "$1" in
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
