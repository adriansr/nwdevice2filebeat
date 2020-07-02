tcp:
host: "{{tcp_host}}:{{tcp_port}}"
tags: {{tags}}
fields_under_root: true
fields:
    observer:
        vendor: ((.Vendor | printf "%q"))
        product: ((.Product | printf "%q"))
        type: ((.Group | printf "%q"))

processors:
((- setvar "basedir" (print "${path.home}/module/" .Module "/" .Fileset) -))
((- setvar "var_prefix" "" -))
((- getvar "extra_processors" -))
- community_id:
- add_locale: ~
- add_fields:
    target: ''
    fields:
        ecs.version: 1.5.0
