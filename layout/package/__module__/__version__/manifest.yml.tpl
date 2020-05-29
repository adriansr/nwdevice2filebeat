format_version: 1.0.0
name: ((.Module))
title: ((.DisplayName))
description: ((.DisplayName)) Integration
version: ((.Version))
categories:
 - logs
release: beta # TODO
removable: true
license: basic
type: integration
requirement:
    kibana:
        versions: '>=7.0.0'
    elasticsearch:
        versions: '>=7.0.0'
datasources:
- name: ((.Module))
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
