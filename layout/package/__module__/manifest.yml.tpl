format_version: 1.0.0
name: ((.Module))
title: ((.DisplayName))
version: ((.Version))
description: ((.DisplayName)) Integration
categories: ((.Categories | tojson))
release: experimental
license: basic
type: integration
conditions:
  kibana.version: '^7.10.0'
policy_templates:
  - name: ((.Fileset))
    title: ((.DisplayName))
    description: Collect ((.DisplayName)) logs from syslog or a file.
    inputs:
      - type: udp
        title: Collect logs from ((.DisplayName)) via UDP
        description: Collecting syslog from ((.DisplayName)) via UDP
      - type: tcp
        title: Collect logs from ((.DisplayName)) via TCP
        description: Collecting syslog from ((.DisplayName)) via TCP
      - type: file
        title: Collect logs from ((.DisplayName)) via file
        description: Collecting syslog from ((.DisplayName)) via file.
  ((- if .Icon ))
icons:
  - src: /img/logo.svg
    title: ((.DisplayName)) logo
    size: 32x32
    type: image/svg+xml
(( end ))
owner:
  github: elastic/security-external-integrations
