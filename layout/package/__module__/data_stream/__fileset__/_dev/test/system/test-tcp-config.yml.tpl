service: ((.Module))-((.Fileset))-tcp
service_notify_signal: SIGHUP
input: tcp
data_stream:
  vars:
    tcp_host: 0.0.0.0
    tcp_port: ((.Port))
