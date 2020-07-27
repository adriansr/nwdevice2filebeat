import re
import sys
import yaml


ws_re = re.compile(r"\s+")


def safe_desc(s):
    s = s.replace('\n', ' ').replace('|', '\|')
    return ws_re.sub(' ', s).strip()


def load_fields(yml, prefix=''):
    output = {}
    assert isinstance(yml, list)
    for fld in yml:
        n = fld['name']
        t = fld['type'] if 'type' in fld else 'keyword'
        d = fld['description'] if 'description' in fld else ''
        if t == 'group' or t == 'object':
            path = prefix + n + '.'
            output.update(load_fields(fld['fields'], prefix=path))
        else:
            output[prefix + n] = (t, safe_desc(d))
    return output


def load_yml(path):
    with open(path, 'r') as f:
        content = yaml.load(f)
        return load_fields(content)


if __name__ == '__main__':
    if len(sys.argv) < 2:
        print('Usage: {} [fields.yml ...]'.format(sys.argv[0]))
        sys.exit(1)
    
    fields = {}
    for path in sys.argv[1:]:
        fields.update(load_yml(path))

print('| Field | Description | Type |')
print('|---|---|---|')

keys = list(fields.keys())
keys.sort()

for k in keys:
    print('| {} | {} | {} |'.format(k, fields[k][1], fields[k][0]))
print('')
