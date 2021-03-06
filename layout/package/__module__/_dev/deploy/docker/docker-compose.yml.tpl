version: '2.3'
services:
  ((.Module))-((.Fileset))-logfile:
    image: alpine
    volumes:
      - ./sample_logs:/sample_logs:ro
      - ${SERVICE_LOGS_DIR}:/var/log
    command: /bin/sh -c "cp /sample_logs/* /var/log/"
  ((.Module))-((.Fileset))-udp:
    image: akroh/stream:v0.0.1
    volumes:
      - ./sample_logs:/sample_logs:ro
    entrypoint: /bin/bash
    command: -c "/stream log --start-signal=SIGHUP --delay=5s --addr elastic-agent:((.Port)) -p=udp /sample_logs/((.Module))-((.Fileset))-*.log"
  ((.Module))-((.Fileset))-tcp:
    image: akroh/stream:v0.0.1
    volumes:
      - ./sample_logs:/sample_logs:ro
    entrypoint: /bin/bash
    command: -c "/stream log --start-signal=SIGHUP --delay=5s --addr elastic-agent:((.Port)) -p=tcp /sample_logs/((.Module))-((.Fileset))-*.log"
