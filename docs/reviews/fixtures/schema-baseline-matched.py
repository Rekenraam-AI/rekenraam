#!/usr/bin/env python3
"""Sequential #405 comparison. BASE HEAD OUTPUT; caller sets GOCACHE.

No competing builds/tests. Output includes raw logs and a compact summary.
MATCHED_RESUME=1 retains successful runs: use only when their source is unchanged.
Changed passing variants require a new output directory and fresh runs.
"""
import datetime,json,os,pathlib,subprocess,sys,time
base,head,out=map(lambda p:pathlib.Path(p).resolve(),sys.argv[1:])
out.mkdir(exist_ok=True,parents=True)
summary={'order':[],'runs':[]}
if os.environ.get('MATCHED_RESUME') == '1':
    summary=json.loads((out/'matched-summary.json').read_text())
def run(name,tree,args,env=None):
    if not name.startswith('compile-') and any(r['name']==name and r['exit']==0 for r in summary['runs']):
        print('retaining successful run',name,flush=True)
        return
    if (out/(name+'.log')).exists():
        (out/(name+'.log')).rename(out/(name+'.previous.log'))
    start=datetime.datetime.now(datetime.timezone.utc).isoformat()
    print(start,name,flush=True)
    with (out/(name+'.log')).open('wb') as log:
        t=time.monotonic()
        result=subprocess.run(args,cwd=tree,env=dict(os.environ,**(env or {})),stdout=log,stderr=subprocess.STDOUT)
        elapsed=time.monotonic()-t
    summary['order'].append(name)
    summary['runs'].append({'name':name,'tree':str(tree),'started_utc':start,'wall_s':round(elapsed,3),'exit':result.returncode})
    (out/'matched-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(name,'exit',result.returncode,'wall_s',round(elapsed,3),flush=True)
    if result.returncode:sys.exit(result.returncode)
for label,tree in [('base',base),('head',head)]:
    run('compile-'+label,tree,['./scripts/go-backend.sh','test','-race','-c','-o',str(out/(label+'-app.test')),'./internal/app'])
for n in [1]:
    for label,tree in [('base',base),('head',head)]:
        run('full-'+label+'-'+str(n),tree,['./scripts/test-backend.sh'],{'GOFLAGS':'-count=1','GO_PHASE_TIMINGS':'1'})
for n in range(1,4):
    for label,tree in [('base',base),('head',head)]:
        name='app-'+label+'-'+str(n)
        # test2json times only the prebuilt test process; no compiler runs here.
        run(name,tree/'backend/internal/app',['go','tool','test2json','-t','-p','rekenraam/backend/internal/app',str(out/(label+'-app.test')),'-test.v=test2json','-test.count=1','-test.timeout=10m'])
for n in [2]:
    for label,tree in [('base',base),('head',head)]:
        run('full-'+label+'-'+str(n),tree,['./scripts/test-backend.sh'],{'GOFLAGS':'-count=1','GO_PHASE_TIMINGS':'1'})
