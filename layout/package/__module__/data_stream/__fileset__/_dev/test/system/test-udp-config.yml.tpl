service: ((.Module))-((.Fileset))-udp
service_notify_signal: SIGHUP
input: udp
data_stream:
  vars:
    udp_host: 0.0.0.0
    udp_port: ((.Port))
