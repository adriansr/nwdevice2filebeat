title: ((.DisplayName)) logs
release: beta
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
      - forwarded
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
  - name: tz_offset
    type: text
    title: Timezone offset (+HH:mm format)
    required: false
    show_user: true
    default: "local"
  - name: rsa_fields
    type: boolean
    title: Add non-ECS fields
    required: false
    show_user: true
    default: true
  - name: keep_raw_fields
    type: boolean
    title: Keep raw parser fields
    required: false
    show_user: true
    default: false
  - name: debug
    type: boolean
    title: Enable debug logging
    required: false
    show_user: true
    default: false

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
      - forwarded
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
  - name: tz_offset
    type: text
    title: Timezone offset (+HH:mm format)
    required: false
    show_user: true
    default: "local"
  - name: rsa_fields
    type: boolean
    title: Add non-ECS fields
    required: false
    show_user: true
    default: true
  - name: keep_raw_fields
    type: boolean
    title: Keep raw parser fields
    required: false
    show_user: true
    default: false
  - name: debug
    type: boolean
    title: Enable debug logging
    required: false
    show_user: true
    default: false

- input: file
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
      - forwarded
  - name: tz_offset
    type: text
    title: Timezone offset (+HH:mm format)
    required: false
    show_user: true
    default: "local"
  - name: rsa_fields
    type: boolean
    title: Add non-ECS fields
    required: false
    show_user: true
    default: true
  - name: keep_raw_fields
    type: boolean
    title: Keep raw parser fields
    required: false
    show_user: true
    default: false
  - name: debug
    type: boolean
    title: Enable debug logging
    required: false
    show_user: true
    default: false
