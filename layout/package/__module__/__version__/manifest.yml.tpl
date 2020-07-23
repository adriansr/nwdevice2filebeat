format_version: 1.0.0
name: ((.Module))
title: ((.DisplayName))
version: ((.Version))
description: ((.DisplayName)) Integration
categories: [ "security" ]
release: experimental
removable: true
license: basic
type: integration
conditions:
    kibana:
        version: '>=7.9.0'
    elasticsearch:
        version: '>=7.9.0'
config_templates:
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
# No icon
icon:
