#!/usr/bin/env python3
"""Read-only Linux/Docker resource and storage recorder, common to V2 A/B/C."""
import argparse, datetime, json, os, pathlib, resource, signal, statistics, subprocess, sys, time

def utc(): return datetime.datetime.now(datetime.timezone.utc).isoformat()
def write(path, value): path.write_text(json.dumps(value, indent=2)+'\n')

class Recorder:
    def __init__(self, args):
        self.a=args; self.out=pathlib.Path(args.output); self.out.mkdir(parents=True, exist_ok=False)
        self.commands=(self.out/'commands.jsonl').open('w'); self.previous={}; self.stop=False
        self.containers=[]; self.mounts=[]; self.ticks=os.sysconf('SC_CLK_TCK'); self.pages=os.sysconf('SC_PAGE_SIZE')
    def command(self, argv):
        start=time.monotonic(); stamp=utc()
        try:
            p=subprocess.run(argv, capture_output=True, text=True, timeout=30)
            result={'returncode':p.returncode,'stdout':p.stdout,'stderr':p.stderr}
        except (OSError,subprocess.TimeoutExpired) as exc: result={'returncode':None,'stdout':'','stderr':str(exc)}
        result.update(command=argv, utc=stamp, duration_seconds=time.monotonic()-start)
        self.commands.write(json.dumps(result)+'\n'); self.commands.flush()
        return result
    def output(self, argv):
        r=self.command(argv); return r['stdout'] if r['returncode']==0 else None
    def discover(self):
        ids=self.output(['docker','ps','-q','--filter','label=com.docker.compose.project='+self.a.project])
        if not ids or not ids.strip(): raise RuntimeError('No running containers for exact project label')
        raw=self.output(['docker','inspect',*ids.split()]); objects=json.loads(raw)
        for c in objects:
            service=c['Config']['Labels'].get('com.docker.compose.service','unknown')
            self.containers.append({'id':c['Id'],'service':service,'name':c['Name'],'image_id':c['Image'],'image':c['Config']['Image'],'limits':{k:c['HostConfig'].get(k) for k in ('NanoCpus','Memory','MemorySwap','CpuQuota','CpuPeriod','CpusetCpus')},'pid':c['State']['Pid']})
            for m in c['Mounts']: self.mounts.append(dict(container=c['Id'],service=service,**{k.lower():v for k,v in m.items()}))
        write(self.out/'containers.json',objects)
    def environment(self):
        commands={'os':['uname','-a'],'distribution':['cat','/etc/os-release'],'cpu':['lscpu'],'memory':['cat','/proc/meminfo'],'disks':['lsblk','-b','-J'],'filesystem':['df','-B1'], 'docker':['docker','version'],'docker_budget':['docker','info','--format','{{json .}}'],'compose':['docker','compose','version'],'git_head':['git','rev-parse','HEAD'],'git_base':['git','rev-parse',self.a.git_base],'git_status':['git','status','--porcelain=v1'],'git_diff':['git','diff','--stat'],'locust':[self.a.locust,'--version']}
        env={k:self.command(v) for k,v in commands.items()}
        versions={}
        for c in self.containers:
            if c['service']=='mongo': cmds=[['mongod','--version'],['/opt/mongot/mongot','--version']]
            elif 'redis' in c['service']: cmds=[['redis-server','--version']]
            elif 'nginx' in c['service']: cmds=[['nginx','-v']]
            else: cmds=[['/market','--help']]
            versions[c['service']]=[self.command(['docker','exec',c['id'],*cmd]) for cmd in cmds]
        env.update(containers=self.containers,mounts=self.mounts,component_versions=versions, application_version='Git commit plus immutable image ID (application has no version flag)', client={'pids':self.a.client_pid,'affinity':sorted(os.sched_getaffinity(0)),'cgroup':pathlib.Path('/proc/self/cgroup').read_text(),'limits':pathlib.Path('/proc/self/limits').read_text(),'cgroup_budget':{name:self.command(['cat','/sys/fs/cgroup'+pathlib.Path('/proc/self/cgroup').read_text().strip().split('::',1)[1]+'/'+name]) for name in ('cpu.max','memory.max','memory.swap.max','cpuset.cpus.effective')},'pid_context':{str(pid):{name:self.command(['cat',f'/proc/{pid}/{name}']) for name in ('cmdline','cgroup','limits','status')} for pid in self.a.client_pid}},collector={'pid':os.getpid(),'python':sys.version,'sampling_seconds':1})
        write(self.out/'environment.json',env)
    def storage(self, when):
        result={'utc':utc(),'mounts':[],'mongodb':None,'redis':{},'search':{'logical_bytes':None,'note':'Search index logical data size has no portable endpoint; allocated/apparent files measured below'}}
        for c in self.containers:
            if c['service']=='mongo':
                r=self.command(['docker','exec',c['id'],'mongosh','--quiet',self.a.database,'--eval','print(JSON.stringify(db.stats(1)))'])
                try: result['mongodb']=json.loads(r['stdout']) if r['returncode']==0 else {'status':'unknown','command_error':r}
                except ValueError: result['mongodb']={'status':'unknown','command_error':r}
            if 'redis' in c['service']:
                result['redis'][c['service']]={'memory':self.command(['docker','exec',c['id'],'redis-cli','INFO','memory']),'persistence':self.command(['docker','exec',c['id'],'redis-cli','INFO','persistence']),'configuration':self.command(['docker','exec',c['id'],'redis-cli','CONFIG','GET','save','appendonly','maxmemory','maxmemory-policy']), 'persistent_mounts':[m for m in self.mounts if m['container']==c['id'] and m['type']=='volume'], 'note':'No named persistent volume' if not any(m['container']==c['id'] and m['type']=='volume' for m in self.mounts) else 'Persistence configuration recorded; mount alone does not imply writes'}
        for m in self.mounts:
            # du allocated vs apparent sizes; errors remain explicit, never zero.
            r=self.command(['docker','exec',m['container'],'du','-s','-k',m['destination']]); a=self.command(['docker','exec',m['container'],'du','-s','-b',m['destination']])
            def size(x):
                try: return int(x['stdout'].split()[0]) if x['returncode']==0 else None
                except (ValueError,IndexError): return None
            result['mounts'].append(dict(m, allocated_bytes=1024*size(r) if size(r) is not None else None,apparent_bytes=size(a),status='measured' if size(r) is not None else 'unknown'))
        write(self.out/('storage-'+when+'.json'),result)
    def process(self,pid,component,container=None,service=None):
        stamp=time.monotonic()
        try:
            fields=pathlib.Path(f'/proc/{pid}/stat').read_text().rsplit(')',1)[1].split()
            cpu=(int(fields[11])+int(fields[12]))/self.ticks; start=fields[19]; rss=int(fields[21])*self.pages
            key=(pid,start); old=self.previous.get(key); self.previous[key]=(stamp,cpu)
            return {'pid':pid,'component':component,'container':container,'service':service,'rss_bytes':rss,'cpu_seconds':cpu,'cpu_percent':100*(cpu-old[1])/(stamp-old[0]) if old else None,'sample_monotonic':stamp,'status':'measured'}
        except (OSError,ValueError,IndexError) as exc: return {'pid':pid,'component':component,'status':'unknown','error':str(exc),'rss_bytes':None,'cpu_percent':None}
    def sample(self):
        begin=time.monotonic(); unix_ns=time.time_ns(); timestamp=utc(); processes=[]
        for c in self.containers:
            top=self.output(['docker','top',c['id'],'-eo','pid,comm,args'])
            if top is None:
                processes.append({'component':c['service'],'container':c['id'],'status':'unknown','error':'docker top failed'}); continue
            for line in top.splitlines()[1:]:
                fields=line.split(None,2)
                if len(fields)<2: continue
                pid=int(fields[0]); comm=fields[1]; args=fields[2] if len(fields)>2 else ''
                kind='mongodb' if comm=='mongod' else 'search' if 'MongotCli' in args else 'redis' if comm.startswith('redis') else 'nginx' if comm=='nginx' else 'application' if comm in ('market','validation') else 'other'
                processes.append(self.process(pid,kind,c['id'],c['service']))
        client_pids=set(self.a.client_pid)
        # Include descendants (uv/runner wrapper is not the actual Locust worker).
        pending=list(client_pids)
        while pending:
            pid=pending.pop()
            try:
                children=pathlib.Path(f'/proc/{pid}/task/{pid}/children').read_text().split()
                for child in map(int,children):
                    if child not in client_pids: client_pids.add(child); pending.append(child)
            except OSError: pass
        processes += [self.process(pid,'client') for pid in sorted(client_pids)]
        processes.append(self.process(os.getpid(),'collector'))
        host={'proc_stat':pathlib.Path('/proc/stat').read_text(),'meminfo':pathlib.Path('/proc/meminfo').read_text(),'loadavg':pathlib.Path('/proc/loadavg').read_text()}
        # --no-stream sleeps ~2 seconds; read container cgroup directly using init PID namespace path instead.
        containers=[]
        for c in self.containers:
            try:
                cg=pathlib.Path(f'/proc/{c["pid"]}/cgroup').read_text().strip().split('::',1)[1]; base=pathlib.Path('/sys/fs/cgroup'+cg)
                containers.append({'id':c['id'],'service':c['service'],'cpu_stat':(base/'cpu.stat').read_text(),'memory_bytes':int((base/'memory.current').read_text()),'memory_peak_bytes':int((base/'memory.peak').read_text()),'memory_stat':(base/'memory.stat').read_text(),'io_stat':(base/'io.stat').read_text(),'status':'measured'})
            except (OSError,ValueError,IndexError) as exc: containers.append({'id':c['id'],'service':c['service'],'status':'unknown','error':str(exc)})
        return {'utc':timestamp,'unix_ns':unix_ns,'monotonic':begin,'phase':self.a.phase,'processes':processes,'containers':containers,'host':host,'collection_seconds':time.monotonic()-begin}
    def run(self):
        self.discover(); self.environment(); self.storage('before')
        signal.signal(signal.SIGTERM,lambda *_:setattr(self,'stop',True)); signal.signal(signal.SIGINT,lambda *_:setattr(self,'stop',True))
        start=time.monotonic(); write(self.out/'ready.json',{'utc':utc(),'unix_ns':time.time_ns(),'monotonic':start,'pid':os.getpid()})
        with (self.out/'samples.jsonl').open('w') as stream:
            next_sample=start
            while not self.stop and (self.a.seconds is None or time.monotonic()-start<self.a.seconds):
                stream.write(json.dumps(self.sample())+'\n'); stream.flush(); next_sample+=1
                time.sleep(max(0,next_sample-time.monotonic()))
        write(self.out/'stopped.json',{'utc':utc(),'unix_ns':time.time_ns(),'monotonic':time.monotonic()})
        self.storage('after'); self.commands.close()
        own=resource.getrusage(resource.RUSAGE_SELF); child=resource.getrusage(resource.RUSAGE_CHILDREN)
        write(self.out/'collector-cost.json',{'self_cpu_seconds':own.ru_utime+own.ru_stime,'children_cpu_seconds':child.ru_utime+child.ru_stime,'self_peak_rss_bytes':own.ru_maxrss*1024,'children_peak_rss_bytes':child.ru_maxrss*1024,'note':'Linux ru_maxrss in KiB; children CPU includes Docker CLI, not daemon/exec-process CPU. Their cost remains in host/container observations.'})
        summarize(self.out)

def summarize(out, window=None):
    samples=[json.loads(line) for line in (out/'samples.jsonl').read_text().splitlines()]
    coverage_window=None
    if window:
        w=json.loads(pathlib.Path(window).read_text()); start=w['startUnixNs']; end=w['deadlineUnixNs']
        coverage_window={'start_unix_ns':start,'end_unix_ns':end,'covered':bool(samples and samples[0]['unix_ns']<=start and samples[-1]['unix_ns']>=end)}
        samples=[s for s in samples if start<=s['unix_ns']<=end]
    result={'schema_version':1,'units':{'cpu_percent':'100% = one logical CPU core; delta user+system process time','rss_bytes':'resident pages; shared pages may overlap across processes','container_memory_bytes':'cgroup current includes file cache; not attributed to MongoDB or Search twice'},'sample_count':len(samples),'first_utc':samples[0]['utc'] if samples else None,'last_utc':samples[-1]['utc'] if samples else None,'components':{},'processes':{},'containers':{},'host':{},'coverage':{'window':coverage_window,'max_gap_seconds':max((b['monotonic']-a['monotonic'] for a,b in zip(samples,samples[1:])),default=None)},'collector':{}}
    def metric(values): return {'samples':len(values),'mean':statistics.mean(values) if values else None,'peak':max(values) if values else None}
    for kind in ('application','mongodb','search','redis','nginx','client','collector','other'):
        found=[p for s in samples for p in s['processes'] if p.get('component')==kind]
        totals=[]; cpus=[]
        for s in samples:
            ps=[p for p in s['processes'] if p.get('component')==kind and p.get('status')=='measured']
            if ps: totals.append(sum(p['rss_bytes'] for p in ps))
            if ps and all(p['cpu_percent'] is not None for p in ps): cpus.append(sum(p['cpu_percent'] for p in ps))
        result['components'][kind]={'status':'measured' if totals else 'unknown' if found else 'absent','rss_bytes':metric(totals),'cpu_percent':metric(cpus),'unavailable_samples':sum(1 for sample in samples if found and (not any(p.get('component')==kind and p.get('status')=='measured' for p in sample['processes']) or any(p.get('component')==kind and p.get('status')!='measured' for p in sample['processes'])))}
    for key in sorted({(p.get('service'),p.get('pid')) for s in samples for p in s['processes'] if p.get('pid')},key=str):
        found=[p for s in samples for p in s['processes'] if (p.get('service'),p.get('pid'))==key]
        result['processes'][f'{key[0]}:{key[1]}']={'component':found[0]['component'],'rss_bytes':metric([p['rss_bytes'] for p in found if p.get('rss_bytes') is not None]),'cpu_percent':metric([p['cpu_percent'] for p in found if p.get('cpu_percent') is not None])}
    for cid in sorted({c['id'] for s in samples for c in s['containers']}):
        found=[c for s in samples for c in s['containers'] if c['id']==cid]
        cpu=[]; previous=None
        for sample in samples:
            c=next(x for x in sample['containers'] if x['id']==cid)
            if c.get('status')!='measured': continue
            usage=int(dict(line.split() for line in c['cpu_stat'].splitlines())['usage_usec'])
            if previous: cpu.append((usage-previous[1])/10000/(sample['monotonic']-previous[0]))
            previous=(sample['monotonic'],usage)
        result['containers'][found[0]['service']]={'id':cid,'memory_bytes':metric([c['memory_bytes'] for c in found if c.get('status')=='measured']),'cpu_percent':metric(cpu)}
    host_cpu=[]; host_mem=[]; previous=None
    for sample in samples:
        fields=list(map(int,sample['host']['proc_stat'].splitlines()[0].split()[1:9])); total=sum(fields); idle=fields[3]+fields[4]
        if previous and total>previous[0]: host_cpu.append(100*(1-(idle-previous[1])/(total-previous[0])))
        previous=(total,idle)
        mem={line.split(':')[0]:int(line.split()[1])*1024 for line in sample['host']['meminfo'].splitlines() if line.split()[-1]=='kB'}
        host_mem.append(mem['MemTotal']-mem['MemAvailable'])
    result['host']={'cpu_percent_of_all_cores':metric(host_cpu),'memory_used_bytes':metric(host_mem)}
    result['collector']={'collection_seconds':metric([s['collection_seconds'] for s in samples]),'cpu_seconds':sum((p.get('cpu_seconds',0) for p in samples[-1]['processes'] if p.get('component')=='collector'),0) if samples else None,'accounting':json.loads((out/'collector-cost.json').read_text()) if (out/'collector-cost.json').exists() else None,'note':'Collector process CPU includes initial environment/storage. docker exec child costs also appear in sampled host/container totals; no background daemon overhead subtraction.'}
    write(out/('summary-window.json' if window else 'summary.json'),result)

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--project'); parser.add_argument('--database'); parser.add_argument('--output',required=True); parser.add_argument('--phase',default='idle'); parser.add_argument('--seconds',type=float); parser.add_argument('--client-pid',type=int,action='append',default=[]); parser.add_argument('--git-base',default='main'); parser.add_argument('--locust',default='locust'); parser.add_argument('--window',help='Formal window.json startUnixNs/deadlineUnixNs; recompute restricted summary'); parser.add_argument('--summarize',action='store_true')
    args=parser.parse_args()
    if args.summarize: summarize(pathlib.Path(args.output),args.window)
    else:
        if not args.project or not args.database: parser.error('--project and --database are required')
        if args.seconds is not None and args.seconds<=0: parser.error('--seconds must be positive')
        Recorder(args).run()
