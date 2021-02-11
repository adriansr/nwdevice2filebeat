paths:
{{#each paths as |path i|}}
  - {{path}}
{{/each}}
exclude_files: [".gz$"]
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
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: dns.question.name
    target_field: dns.question.registered_domain
    target_subdomain_field: dns.question.subdomain
    target_etld_field: dns.question.top_level_domain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: client.domain
    target_field: client.registered_domain
    target_subdomain_field: client.subdomain
    target_etld_field: client.top_level_domain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: server.domain
    target_field: server.registered_domain
    target_subdomain_field: server.subdomain
    target_etld_field: server.top_level_domain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: destination.domain
    target_field: destination.registered_domain
    target_subdomain_field: destination.subdomain
    target_etld_field: destination.top_level_domain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: source.domain
    target_field: source.registered_domain
    target_subdomain_field: source.subdomain
    target_etld_field: source.top_level_domain
- registered_domain:
    ignore_missing: true
    ignore_failure: true
    field: url.domain
    target_field: url.registered_domain
    target_subdomain_field: url.subdomain
    target_etld_field: url.top_level_domain
- add_locale: ~
- add_fields:
    target: ''
    fields:
        ecs.version: 1.8.0
