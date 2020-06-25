# Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
# or more contributor license agreements. Licensed under the Elastic License;
# you may not use this file except in compliance with the Elastic License.

import csv
import sys

from shared import *


class Setter:
    def __init__(self, src, dst, mode, conv=None, prio=None):
        self.src = src
        self.dst = dst
        self.mode = mode
        self.conv = conv
        self.prio = prio

    def __str__(self):
        if self.mode == 'prio':
            return '{{field: "{}", setter: fld_{}, prio: {}}}'.format(self.dst, self.mode, self.prio)
        return '{{field: "{}", setter: fld_{}}}'.format(self.dst, self.mode)


# From highest priority to lowest priority
def by_prio(lst):
    return {
        'mode': 'prio',
        'priorities': {x[1]: x[0] for x in enumerate(lst)}
    }


append = {'mode': 'append'}


def process_row(row):
    lst = filter(str.__len__, [row[idx] for idx in [MAP, ALT]])
    typ = row[TYPE]
    if typ not in type_to_es:
        raise Exception('unsupported type: {}'.format(typ))
    conv = type_to_es[typ]
    return row[SRC], [Setter(row[SRC], field, 'set', conv) for field in lst]


xsetters = {
    'go': lambda x: x.as_go,
    'js': lambda x: x.as_js,
}

if len(sys.argv) < 3 or len(sys.argv) > 4 or sys.argv[1] not in xsetters:
    print('Usage: {} {{go|js}} file.csv [overrides.csv]'.format(sys.argv[0]))
    sys.exit(1)

xsetter = xsetters[sys.argv[1]]

overrides = {}
if len(sys.argv) == 4:
    with open(sys.argv[3]) as f:
        r = csv.reader(f, dialect=csv.excel)
        for row in r:
            if row[0] in overrides:
                raise('Repeated override entry: {}'.format(row[0]))
            if row[1] == 'append':
                if len(row) > 2:
                    raise('Excess data after append override: {}'.format(row[2:]))
                overrides[row[0]] = append
            elif row[1] == 'by_prio':
                if len(row) < 4:
                    raise('Need at least 2 fields for by_prio override: {}'.format(row[2:]))
                overrides[row[0]] = by_prio(row[2:])

f = open(sys.argv[2], 'r')
r = csv.reader(f, dialect=csv.excel)
first = True
by_dst = {}
by_src = {}
for row in r:
    if first and row[0] == 'revision':
        # skip header
        first = False
        continue
    fld, mappings = process_row(row)
    if fld in by_src:
        raise Exception('Repeated field: {}'.format(fld))
    # Add ip fields to related.ip if they map to any ECS field.
    if len(mappings) > 0 and mappings[0].conv == "ip" and any(map(lambda x: is_ecs_field(x.dst), mappings)):
        mappings.append(Setter(fld, "related.ip", "append", "ip"))
    by_src[fld] = mappings
    for m in mappings:
        if m.dst in overrides:
            o = overrides[m.dst]
            m.mode = o['mode']
            if m.mode == 'prio':
                if m.src not in o['priorities']:
                    raise Exception('No priority for src:{} dst:{}'.format(m.src, m.dst))
                m.prio = o['priorities'][m.src]
        if m.dst not in by_dst:
            by_dst[m.dst] = []
        by_dst[m.dst].append(m)

for dst, setters in by_dst.items():
    modes = set([s.mode for s in setters])
    if len(modes) != 1:
        raise Exception('Field {} is set in different modes: {}'.format(dst, modes))
    convs = set([s.conv for s in setters])
    if len(convs) != 1:
        raise Exception('Field {} is set from different types: {}'.format(dst, convs))
    if len(setters) > 1 and setters[0].mode == 'set':
        raise Exception('Field {} is set multiple times. Must override mode (sources=[{}])'.format(
            dst,
            ', '.join(['"'+x.src+'"' for x in setters])))

for src, setters in by_src.items():
    convs = set([s.conv for s in setters])
    if len(convs) != 1:
        raise Exception('Field {} is converted to different types: {}'.format(src, convs))

def dump_setters(by_src, filter_fn):
    items = list(by_src.items())
    items.sort()
    for src, setters in items:
        setters = list(filter(lambda x: filter_fn(x.dst), setters))
        if len(setters) == 0:
            continue
        conv = ''
        if setters[0].conv is not None:
            conv = 'convert: to_{}, '.format(setters[0].conv)
        print('    "{}": {{{}to:[{}]}},'.format(src, conv, ','.join(map(str, setters))))


print('var ecs_mappings = {')
dump_setters(by_src, is_ecs_field)
print('};')
print('')
print('var rsa_mappings = {')
dump_setters(by_src, is_rsa_field)
print('};')
