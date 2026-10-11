#!/usr/bin/env python3
"""Wait for advertised Mongo + actual Search management in an owned V2 project."""
import argparse
import json
import re
import secrets
import subprocess
import time

parser = argparse.ArgumentParser()
parser.add_argument('--project', required=True)
parser.add_argument('--timeout', type=float, default=120)
args = parser.parse_args()
if not re.fullmatch(r'campus-v2-[a-z0-9-]+', args.project) or args.timeout <= 0:
    parser.error('an isolated campus-v2-* project and positive timeout are required')
compose = ['docker', 'compose', '-p', args.project, '-f', 'compose.v2.yaml']
container = subprocess.check_output(compose + ['ps', '-q', 'mongo'], text=True).strip()
if not container or '\n' in container:
    raise SystemExit('exactly one running owned Mongo service is required')
owner = subprocess.check_output(['docker', 'inspect', container, '--format', '{{index .Config.Labels "com.docker.compose.project"}}'], text=True).strip()
if owner != args.project:
    raise SystemExit('Mongo service ownership does not match requested project')
token = secrets.token_hex(16)
name = 'campus_v2_readiness_' + token
prefix = f'const d=db.getSiblingDB({json.dumps(name)});const token={json.dumps(token)};'
probe = prefix + '''
const parameters=db.getSiblingDB('admin').runCommand({getParameter:1,searchIndexManagementHostAndPort:1,mongotHost:1});
if(!parameters.ok||!parameters.searchIndexManagementHostAndPort||!parameters.mongotHost)throw Error('Atlas Search hosts not configured yet');
if(d.getCollectionNames().length===0)d.products.insertOne({_id:'owner',token:token});
const owner=d.products.findOne({_id:'owner'});
if(!owner||owner.token!==token){print('readiness DB collision; refusing mutation');quit(2);}
print(JSON.stringify(d.products.aggregate([{$listSearchIndexes:{name:'products_v1'}}]).toArray()));
'''
cleanup = prefix + '''
if(d.getCollectionNames().length!==0){const owner=d.products.findOne({_id:'owner'});if(!owner||owner.token!==token){print('readiness DB ownership mismatch; refusing drop');quit(2);}print(JSON.stringify(d.dropDatabase()));}
'''
command = compose + ['exec', '-T', 'mongo', 'mongosh', 'mongodb://mongo:27017/?directConnection=true&serverSelectionTimeoutMS=2000&connectTimeoutMS=2000', '--quiet', '--eval']
def run(code):
    result = subprocess.run(command + [code], text=True, capture_output=True, timeout=10)
    print(json.dumps({'at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()), 'database': name, 'returnCode': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr}), flush=True)
    return result.returncode
passed = False
consecutive = 0
deadline = time.monotonic() + args.timeout
try:
    while time.monotonic() < deadline:
        code = run(probe)
        if code == 0:
            consecutive += 1
            if consecutive >= 3:
                passed = True
                break
        else:
            consecutive = 0
        if code == 2:
            raise SystemExit('readiness namespace collision')
        time.sleep(min(2, max(0, deadline-time.monotonic())))
finally:
    if run(cleanup) != 0:
        raise SystemExit('readiness cleanup failed; see owned project/database above')
if not passed:
    raise SystemExit('Mongo/Search management readiness deadline exceeded')
print(json.dumps({'result': 'ready', 'project': args.project, 'probeDatabaseDropped': name}), flush=True)
