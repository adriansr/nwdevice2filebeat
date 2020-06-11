import csv
import sys

SRC=4
TYPE=6
MAP=11
ALT=12

def is_rsa_field(path):
    return path.startswith('rsa.')

def is_ecs_field(path):
    return not is_rsa_field(path)

class GoFormat:
    def format(self, fld):
        if is_rsa_field(fld):
            parts = fld.split('.')
            assert len(parts) == 3, fld
            return 'custom{{\"{}\", \"{}\"}}'.format(parts[1], parts[2])
        return 'ecs{{\"{}\"}}'.format(fld)

    def output(self, row):
        lst = []
        for idx in [MAP, ALT]:
            if row[idx] != '':
                lst.append(self.format(row[idx]))
        
        typ = row[TYPE]
        print '\"{}\": {{Type: {}, Map: []mapper{{{}}}}},'.format(row[SRC], typ, ', '.join(lst))


class JSFormat:
    conv = {
        '': None,
        'Text': None, # Default -- keyword
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

    def output(self, row):
        lst = filter(str.__len__, [ row[idx] for idx in [MAP, ALT] ])
        ecs = filter(is_ecs_field, lst)
        rsa = filter(is_rsa_field, lst)

        typ = row[TYPE]
        if typ not in self.conv:
            raise Exception('unsupported type: {}'.format(typ))
        parts = []
        conv = self.conv[typ]
        if conv is not None:
            parts.append('convert: to_{}'.format(conv))
        if len(ecs):
            parts.append('ecs: [{}]'.format(', '.join(map(str.__repr__, ecs))))
        if len(rsa):
            parts.append('rsa: [{}]'.format(', '.join(map(str.__repr__, rsa))))

        print '{}: {{{}}},'.format(row[SRC].__repr__(), ', '.join(parts))


if len(sys.argv) != 3 or (sys.argv[1]!='go' and sys.argv[1]!='js'):
    print 'Usage: {} {{go|js}} file.csv'.format(sys.argv[0])
    sys.exit(1)


fmt = GoFormat()
if sys.argv[1] == 'js':
    fmt = JSFormat()

f = open(sys.argv[2], 'r')
r = csv.reader(f, dialect=csv.excel)
first = True
for row in r:
    if first and row[0] == 'revision':
        # skip header
        first = False
        continue
    fmt.output(row)
