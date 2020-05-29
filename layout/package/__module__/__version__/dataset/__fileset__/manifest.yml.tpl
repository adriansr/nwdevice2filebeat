title: ((.DisplayName)) logs
release: ga
type: logs
streams:
- input: udp
  title: ((.DisplayName)) logs
  description: Collect ((.DisplayName)) logs
  template_path: udp.yml.hbs
  vars:
  - name: tags
    type: text
    title: Tags
    multi: true
    required: true
    show_user: false
    default:
      - ((.Module))-((.Fileset))
  - name: udp_host
    type: text
    title: UDP host to listen on
    multi: false
    required: true
    show_user: true
    default: localhost
  - name: udp_port
    type: integer
    title: UDP Port to listen on
    multi: false
    required: true
    show_user: true
    default: ((.Port))
- input: tcp
  title: ((.DisplayName)) logs
  description: Collect ((.DisplayName)) logs
  template_path: tcp.yml.hbs
  vars:
  - name: tags
    type: text
    title: Tags
    multi: true
    required: true
    show_user: false
    default:
      - ((.Module))-((.Fileset))
  - name: tcp_host
    type: text
    title: TCP host to listen on
    multi: false
    required: true
    show_user: true
    default: localhost
  - name: tcp_port
    type: integer
    title: TCP Port to listen on
    multi: false
    required: true
    show_user: true
    default: ((.Port))
- input: logs
  title: ((.DisplayName)) logs
  description: Collect ((.DisplayName)) logs from file
  vars:
  - name: paths
    type: text
    title: Paths
    multi: true
    required: true
    show_user: true
    default:
      - /var/log/((.Module))-((.Fileset)).log
  - name: tags
    type: text
    title: Tags
    multi: true
    required: true
    show_user: false
    default:
      - ((.Module))-((.Fileset))
