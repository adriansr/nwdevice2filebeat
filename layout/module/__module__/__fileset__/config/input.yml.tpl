{{ if eq .input "file" }}

type: log
paths:
  {{ range $i, $path := .paths }}
- {{$path}}
  {{ end }}
exclude_files: [".gz$"]

{{ else }}

type: {{.input}}
host: "{{.syslog_host}}:{{.syslog_port}}"

{{ end }}

tags: {{.tags | tojson}}
publisher_pipeline.disable_host: {{ inList .tags "forwarded" }}

fields_under_root: true
fields:
    observer:
        vendor: ((.Vendor | printf "%q"))
        product: ((.Product | printf "%q"))
        type: ((.Group | printf "%q"))

processors:
((- setvar "basedir" (print "${path.home}/module/" .Module) -))
((- getvar "extra_processors" -))
- community_id: ~
- add_fields:
    target: ''
    fields:
        ecs.version: 1.5.0
