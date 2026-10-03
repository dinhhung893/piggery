#!/bin/bash
# setup.sh <name>: an isolated omp environment in /tmp/pg-omp/<name>: its own HOME (so ~/.piggery,
# ~/.claude, ~/.omp are all inside it), an omp agent dir with only HP/glm-5.3-flash, a private
# piggery binary on PATH. Prints the directory. Never prints the key.
set -e
N=$1; T=/tmp/pg-omp/$N; rm -rf $T; mkdir -p $T/bin $T/agent $T/proj
cp /tmp/pg-omp/piggery $T/bin/piggery
python3 - $T <<'EOF'
import json,os,sys
d=json.load(open(os.path.expanduser('~/.pi/agent/models.json')))
p=(d.get('providers') or d)['HP']
m=[x for x in p['models'] if x['id']=='glm-5.3-flash'][0]
def y(v,ind=0):
    return json.dumps(v)   # JSON is valid YAML
prov={"baseUrl":p['baseUrl'],"api":p['api'],"apiKey":p['apiKey'],
      "models":[{k:m[k] for k in m if k in('id','name','reasoning','input','contextWindow','maxTokens')}]}
open(sys.argv[1]+'/agent/models.yml','w').write(json.dumps({"providers":{"HP":prov}},indent=1))
os.chmod(sys.argv[1]+'/agent/models.yml',0o600)
EOF
echo $T
