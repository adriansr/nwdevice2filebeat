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
((- setvar "var_prefix" "." -))
((- getvar "extra_processors" -))
{{ if .community_id }}
- community_id: ~
{{ end }}
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: dns.question.name
    target_field: dns.question.registered_domain
    target_subdomain_field: dns.question.subdomain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: client.domain
    target_field: client.registered_domain
    target_subdomain_field: client.subdomain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: server.domain
    target_field: server.registered_domain
    target_subdomain_field: server.subdomain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: destination.domain
    target_field: destination.registered_domain
    target_subdomain_field: destination.subdomain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: source.domain
    target_field: source.registered_domain
    target_subdomain_field: source.subdomain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: url.domain
    target_field: url.registered_domain
    target_subdomain_field: url.subdomain
- add_fields:
    target: ''
    fields:
        ecs.version: 1.7.0
