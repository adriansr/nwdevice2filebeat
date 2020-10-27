tcp:
host: "{{tcp_host}}:{{tcp_port}}"
tags:
{{#each tags as |tag i|}}
 - {{tag}}
{{/each}}
fields_under_root: true
fields:
    observer:
        vendor: ((.Vendor | printf "%q"))
        product: ((.Product | printf "%q"))
        type: ((.Group | printf "%q"))
{{#contains tags "forwarded"}}
publisher_pipeline.disable_host: true
{{/contains}}

processors:
((- setvar "basedir" (print "${path.home}/module/" .Module "/" .Fileset) -))
((- setvar "var_prefix" "" -))
((- getvar "extra_processors" -))
- community_id:
- add_locale: ~
- add_fields:
    target: ''
    fields:
        ecs.version: 1.6.0
