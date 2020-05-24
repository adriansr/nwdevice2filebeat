module_version: "1.0"

var:
  - name: paths
  - name: tags
    default: [((.Module)).((.Fileset))]
  - name: syslog_host
    default: localhost
  - name: syslog_port
    default: ((.Port))
  - name: input
    default: udp
  - name: community_id
    default: true
 
ingest_pipeline: ingest/pipeline.yml
input: config/input.yml

requires.processors:
- name: geoip
  plugin: ingest-geoip
- name: user_agent
  plugin: ingest-user_agent
