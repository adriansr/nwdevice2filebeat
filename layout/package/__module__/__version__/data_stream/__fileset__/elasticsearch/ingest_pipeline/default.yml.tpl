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
        value: '{{host.name}}'
        allow_duplicates: false
        if: ctx.host?.name != null && ctx.host?.name != ''
on_failure:
  - append:
        field: error.message
        value: "{{ _ingest.on_failure_message }}"
