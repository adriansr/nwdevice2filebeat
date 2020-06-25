# Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
# or more contributor license agreements. Licensed under the Elastic License;
# you may not use this file except in compliance with the Elastic License.

# Columns
DESC = 3
SRC = 4
TYPE = 6
MAP = 11
ALT = 12

# Conversions
type_to_es = {
    '': None,
    'Text': None,  # Default -- keyword
    'TimeT': 'date',
    'IPv4': 'ip',
    'IPv6': 'ip',
    'UInt64': 'long',
    'UInt32': 'long',
    'UInt16': 'long',
    'UInt8': 'long',
    'Int64': 'long',
    'Int32': 'long',
    'Int16': 'long',
    'Float64': 'double',
    'Float32': 'double',
    'MAC': 'mac',
}


def is_rsa_field(path):
    return path.startswith('rsa.')


def is_ecs_field(path):
    return not is_rsa_field(path)
