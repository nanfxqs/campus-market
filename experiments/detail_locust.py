"""Fixed paired detail sequences: 50 serial users, preparation barrier, deadline."""
import gzip,json,os,time
from pathlib import Path
import gevent
from gevent.event import Event
from locust import HttpUser,constant,events,task
from locust.exception import StopUser
OUT=Path(os.environ['DETAIL_OUTPUT']);POOL=json.loads(gzip.decompress((OUT/'pool.json.gz').read_bytes()))['ids'];DURATION=float(os.environ['DETAIL_DURATION'])
READY=Event();NEXT=0;READIED=[];START=None
WINDOW={'schemaVersion':1,'readyUsers':0,'expectedUsers':50,'durationSeconds':DURATION,'formal':os.environ['DETAIL_FORMAL']=='1','preparationStartedUnixNs':time.time_ns(),'finalized':False}
WRITER=gzip.open(OUT/'requests.jsonl.gz','wt',compresslevel=3)
(OUT/'client-pid.json').write_text(json.dumps({'pid':os.getpid(),'parentPid':os.getppid()})+'\n')
def save(): (OUT/'window.json').write_text(json.dumps(WINDOW,indent=2)+'\n')
@events.test_start.add_listener
def start(environment,**_):
    save()
    deadline=time.monotonic()+120
    while not (OUT/'resources/ready.json').exists():
        if time.monotonic()>deadline: raise TimeoutError('resource collector not ready')
        gevent.sleep(.1)
    def watchdog():
        gevent.sleep(60)
        if not READY.is_set():
            WINDOW['error']='50-user readiness timeout';save();environment.process_exit_code=2;environment.runner.quit()
    gevent.spawn(watchdog)
@events.quitting.add_listener
def finish(environment,**_):
    WINDOW['finishedUnixNs']=time.time_ns();WINDOW['finalized']=READY.is_set() and bool(WINDOW.get('deadlineReachedUnixNs'));save();WRITER.close()
    if not WINDOW['finalized']:environment.process_exit_code=2
@events.user_error.add_listener
def error(user_instance,exception,tb,**_):
    WINDOW.setdefault('runtimeErrors',[]).append(str(exception));save();user_instance.environment.process_exit_code=2
class Reader(HttpUser):
    wait_time=constant(0)
    def on_start(self):
        global NEXT,START
        self.index=NEXT;NEXT+=1;self.sequence=0
        if self.index>=50:raise ValueError('exactly 50 users required')
        with self.client.get('/health',name='/health (prepare)',timeout=10,catch_response=True) as r:
            if r.status_code!=200:r.failure('unhealthy');raise StopUser()
        READIED.append(self.index)
        if len(READIED)==50:
            self.environment.runner.stats.reset_all();START=time.monotonic();WINDOW.update(readyUsers=50,startUnixNs=time.time_ns(),startMonotonic=START);WINDOW['deadlineUnixNs']=WINDOW['startUnixNs']+int(DURATION*1e9);WINDOW['preparationSeconds']=(WINDOW['startUnixNs']-WINDOW['preparationStartedUnixNs'])/1e9;save();READY.set()
            def stop():
                gevent.sleep(max(0,START+DURATION-time.monotonic()));WINDOW['deadlineReachedUnixNs']=time.time_ns();save();self.environment.runner.quit()
            gevent.spawn(stop)
        READY.wait()
    @task
    def detail(self):
        if time.monotonic()>=START+DURATION:raise StopUser()
        position=(self.index+50*self.sequence)%len(POOL);product=POOL[position];launched=time.monotonic();unix=time.time_ns()
        with self.client.get('/products/'+product,name='/products/:id',timeout=10,catch_response=True) as r:
            received=time.monotonic();status=r.status_code or 0;valid=False;telemetry={}
            try:
                data=r.json();valid=status==200 and data['product']['id']==product and isinstance(data.get('seller'),dict) and isinstance(data.get('available'),bool)
                telemetry={'cache':r.headers['X-Detail-Cache'],'mongoQueries':int(r.headers['X-Mongo-Queries']),'mongoBsonBytes':int(r.headers['X-Mongo-BSON-Bytes']),'cacheErrors':0 if os.environ['DETAIL_CONFIGURATION']=='A' else int(r.headers['X-Detail-Cache-Errors'])}
                valid=valid and telemetry['cache'] in ('disabled','hit','miss','fallback') and telemetry['mongoQueries']>0 and telemetry['mongoBsonBytes']>0
            except (ValueError,KeyError,TypeError):pass
            if valid:r.success()
            else:r.failure('invalid HTTP detail or missing telemetry')
            WRITER.write(json.dumps({'user':self.index,'sequenceIndex':self.sequence,'poolPosition':position,'productId':product,'startedUnixNs':unix,'receivedUnixNs':time.time_ns(),'startedOffset':launched-START,'receivedOffset':received-START,'elapsedMs':(received-launched)*1000,'status':status,'valid':valid,'telemetry':telemetry,'error':str(getattr(r,'error','') or '')},separators=(',',':'))+'\n')
        self.sequence+=1
