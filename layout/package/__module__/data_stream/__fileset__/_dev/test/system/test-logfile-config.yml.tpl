service: ((.Module))-((.Fileset))-logfile
input: logfile
data_stream:
  vars:
    paths:
      - "{{SERVICE_LOGS_DIR}}/((.Module))-((.Fileset))-*.log"
