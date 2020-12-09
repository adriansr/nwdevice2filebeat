---
description: Pipeline for ((.DisplayName))

processors:
  # ECS event.ingested
  - set:
        field: event.ingested
        value: '{{_ingest.timestamp}}'
  # User agent
  - user_agent:
        field: user_agent.original
        ignore_missing: true
  # IP Geolocation Lookup
  - geoip:
        field: source.ip
        target_field: source.geo
        ignore_missing: true
  - geoip:
        field: destination.ip
        target_field: destination.geo
        ignore_missing: true

  # IP Autonomous System (AS) Lookup
  - geoip:
        database_file: GeoLite2-ASN.mmdb
        field: source.ip
        target_field: source.as
        properties:
            - asn
            - organization_name
        ignore_missing: true
  - geoip:
        database_file: GeoLite2-ASN.mmdb
        field: destination.ip
        target_field: destination.as
        properties:
            - asn
            - organization_name
        ignore_missing: true
  - rename:
        field: source.as.asn
        target_field: source.as.number
        ignore_missing: true
  - rename:
        field: source.as.organization_name
        target_field: source.as.organization.name
        ignore_missing: true
  - rename:
        field: destination.as.asn
        target_field: destination.as.number
        ignore_missing: true
  - rename:
        field: destination.as.organization_name
        target_field: destination.as.organization.name
        ignore_missing: true
  - append:
        field: related.hosts
        value: '{{url.domain}}'
        allow_duplicates: false
        if: ctx?.url?.domain != null && ctx?.url?.domain != ""  
  - append:
        field: related.hosts
        value: '{{server.domain}}'
        allow_duplicates: false
        if: ctx?.server?.domain != null && ctx?.server?.domain != ""  
  - append:
        field: related.hosts
        value: '{{host.name}}'
        allow_duplicates: false
        if: ctx?.host?.name != null && ctx.host?.name != ''
  - append:
        field: related.hosts
        value: '{{host.hostname}}'
        allow_duplicates: false
        if: ctx?.host?.hostnamename != null && ctx.host?.hostname != ''
  - append:
        field: related.hosts
        value: '{{destination.address}}'
        allow_duplicates: false
        if: ctx?.destination?.address != null && ctx.destination?.address != ''
  - append:
        field: related.hosts
        value: '{{source.address}}'
        allow_duplicates: false
        if: ctx?.source?.address != null && ctx.source?.address != ''
  - append:
        field: related.hosts
        value: '{{rsa.web.fqdn}}'
        allow_duplicates: false
        if: ctx?.rsa?.web?.fqdn != null && ctx.rsa?.web?.fqdn != ''
  - append:
        field: related.hosts
        value: '{{rsa.misc.event_source}}'
        allow_duplicates: false
        if: ctx?.rsa?.misc?.event_source != null && ctx.rsa?.misc?.event_source != ''
  - append:
        field: related.hosts
        value: '{{rsa.web.web_ref_domain}}'
        allow_duplicates: false
        if: ctx?.rsa?.web?.web_ref_domain != null && ctx?.rsa?.web?.web_ref_domain != ''
on_failure:
  - append:
        field: error.message
        value: "{{ _ingest.on_failure_message }}"
