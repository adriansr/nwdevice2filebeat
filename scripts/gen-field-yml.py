# Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
# or more contributor license agreements. Licensed under the Elastic License;
# you may not use this file except in compliance with the Elastic License.

import csv
import sys
import yaml
from shared import *


def read_ecs_fields(f):
    def flatten(fields):
        output = {}
        assert isinstance(fields, list)
        for field in fields:
            assert 'name' in field, "no name in field {}".format(field)
            assert 'type' in field, "no type in field {}".format(field)
            name = field['name']
            typ = field['type']
            if typ == 'group' and 'fields' in field:
                inner = flatten(field['fields'])
                output.update(dict([(name + '.' + x[0], x[1]) for x in inner.items()]))
            else:
                output[field['name']] = field
        return output

    content = yaml.load(f)
    assert len(content) == 1, "fields.ecs.yml from ECS project should only have one key"
    assert content[0]['key'] == 'ecs', "fields.ecs.yml from ECS project should start with the ecs key"
    return flatten(content[0]['fields'])


def serialize(fields):
    as_dict = {}
    for name, desc in fields.items():
        components = name.split('.')
        last = components[-1]
        components = components[:-1]
        base = as_dict
        for comp in components:
            if comp in base:
                base = base[comp]['fields']
            else:
                base[comp] = {
                    'name': comp,
                    'type': 'group',
                    'fields': {},
                }
                base = base[comp]['fields']
        if last in base:
            raise Exception('Repeated field ' + name)
        desc['name'] = last
        base[last] = desc
    def to_list(dct):
        result = []
        for _, v in dct.items():
            if 'fields' in v:
                v['fields'] = to_list(v['fields'])
            result.append(v)
        return result
    return to_list(as_dict)


if len(sys.argv) != 3:
    print('Usage: {} <fields.ecs.yml> <mappings.csv>'.format(sys.argv[0]))
    sys.exit(1)

with open(sys.argv[1], 'r') as f:
    ecs_ref = read_ecs_fields(f)

rsa = {}
ecs = {}
with open(sys.argv[2], 'r') as f:
    r = csv.reader(f, dialect=csv.excel)
    first = True
    for row in r:
        if first and row[0] == 'revision':
            # skip header
            first = False
            continue
        typ = row[TYPE]
        if typ not in type_to_es:
            raise Exception('unsupported type: {} in {}'.format(typ, row))
        es_type = type_to_es[typ]
        if es_type is None or es_type is 'mac':
            es_type = 'keyword'
        for field in filter(str.__len__, [row[idx] for idx in [MAP, ALT]]):
            if is_rsa_field(field):
                if field in rsa:
                    if rsa[field]['type'] != es_type:
                        raise Exception('duplicated rsa field "{}" with different types: {} and {}'.format(field, rsa[field], es_type))
                    print('WARNING: Duplicated RSA field ' + field)
                else:
                    rsa[field] = {'name': field, 'type': es_type}
                    if len(row[DESC]) != 0:
                        rsa[field]['description'] = row[DESC]
            else:
                if field in ecs:
                    if ecs[field]['type'] != es_type:
                        raise Exception('duplicated ECS field "{}" with different types: {} and {}'.format(field, ecs[field]['type'], es_type))
                    continue
                if field in ecs_ref:
                    ref = ecs_ref[field]
                else:
                    print('WARNING: Undocumented ECS field ' + field)
                    ref = {'type': es_type}
                ecs[field] = ref

for action in [(rsa, 'fields.yml'), (ecs, 'ecs.yml')]:
    print('Saving {} ...'.format(action[1]))
    with open(action[1], 'w') as f:
        yaml.dump(serialize(action[0]), stream=f, default_flow_style=False, sort_keys=False)

