"""50 serial seller users, a real preparation barrier, and an immutable deadline."""
import gzip
import json
import os
from pathlib import Path
import time
from datetime import datetime

import gevent
from gevent.event import Event
from locust import HttpUser, constant, events, task
from locust.exception import StopUser

ROOT = Path(os.environ['SALE_RESULTS'])
MANIFEST = json.loads(Path(os.environ['SALE_MANIFEST']).read_text())
DURATION = float(os.environ.get('SALE_DURATION','600'))
TARGET = int(os.environ.get('SALE_PRODUCTS','30000'))
TIMEOUT = float(os.environ.get('SALE_TIMEOUT','25'))
RETRIES = int(os.environ.get('SALE_RETRIES','2'))
EXPECTED_USERS = 50
SELECTED = MANIFEST['products'][:TARGET]
ACCOUNTS = {account['id']:account for account in MANIFEST['accounts']}
SELLERS = sorted({row['sellerId'] for row in SELECTED})
if len(SELLERS)!=EXPECTED_USERS or len(SELECTED)!=TARGET:
    raise ValueError('load requires 50 existing designated sellers and enough manifest products')
QUEUES = {seller:[row for row in SELECTED if row['sellerId']==seller] for seller in SELLERS}
READY = Event()
NEXT_USER = 0
READY_USERS = []
EXHAUSTED_USERS = set()
WINDOW = {'schemaVersion':1,'durationSeconds':DURATION,'expectedUsers':EXPECTED_USERS,'readyUsers':0,'formal':os.environ.get('SALE_FORMAL')=='1','selectedProducts':[row['productId'] for row in SELECTED],'preparationStartedUnixNs':time.time_ns(),'finalized':False}
START = None
(ROOT/'client-pid.json').write_text(json.dumps({'pid':os.getpid(),'parentPid':os.getppid(),'startedUnixNs':time.time_ns()})+'\n')
WRITER = gzip.open(ROOT/'requests.jsonl.gz','wt',compresslevel=3)


def save_window():
    (ROOT/'window.json').write_text(json.dumps(WINDOW,indent=2)+'\n')


def request_stop(environment, reason):
    if WINDOW.get('stopReason'): return
    WINDOW['stopReason']=reason
    WINDOW['measuredWindowSeconds']=min(DURATION,time.monotonic()-START)
    WINDOW['measurementEndedUnixNs']=time.time_ns()
    save_window()
    gevent.spawn(environment.runner.quit)


def stop_after_deadline(environment):
    gevent.sleep(max(0,START+DURATION-time.monotonic()))
    WINDOW['deadlineReachedUnixNs']=time.time_ns()
    request_stop(environment,'deadline')


@events.test_start.add_listener
def start_watchdog(environment, **kwargs):
    save_window()
    gate=os.environ.get('SALE_MEASUREMENT_GATE')
    if gate:
        deadline=time.monotonic()+120
        while not Path(gate).exists():
            if time.monotonic()>=deadline: raise RuntimeError('collector start gate timed out')
            gevent.sleep(.1)
        WINDOW['measurementGateReadyUnixNs']=time.time_ns()
        save_window()
    def check_preparation():
        gevent.sleep(float(os.environ.get('SALE_PREPARATION_TIMEOUT','120')))
        if not READY.is_set():
            WINDOW['preparationError']='not all 50 users became ready before preparation deadline'
            save_window()
            environment.process_exit_code=2
            environment.runner.quit()
    gevent.spawn(check_preparation)


@events.user_error.add_listener
def runtime_error(user_instance,exception,tb,**kwargs):
    WINDOW.setdefault("runtimeErrors",[]).append(str(exception))
    save_window()
    user_instance.environment.process_exit_code=2


@events.quitting.add_listener
def finish(environment, **kwargs):
    WINDOW['finishedUnixNs']=time.time_ns()
    WINDOW['readyUsers']=len(READY_USERS)
    WINDOW['finalized']=READY.is_set() and WINDOW.get('stopReason') in ['deadline','pool_exhausted']
    save_window()
    WRITER.close()
    if not WINDOW['finalized']:
        environment.process_exit_code=2


class Seller(HttpUser):
    wait_time = constant(0)

    def on_start(self):
        global NEXT_USER, START
        index=NEXT_USER
        NEXT_USER+=1
        if index>=EXPECTED_USERS:
            raise ValueError('exactly 50 virtual users required')
        self.seller=SELLERS[index]
        self.products=iter(QUEUES[self.seller])
        account=ACCOUNTS[self.seller]
        with self.client.post('/auth/login',json={'username':account['username'],'password':account['password']},name='/auth/login (preparation)',timeout=(min(5,TIMEOUT),TIMEOUT),catch_response=True) as response:
            try:
                payload=response.json()
                expiry=datetime.fromisoformat(payload['expiresAt'].replace('Z','+00:00')).timestamp()
                if response.status_code!=200 or expiry < time.time()+DURATION+TIMEOUT+120:
                    raise ValueError('login failed or token does not cover entire load window')
                self.client.headers['Authorization']='Bearer '+payload['accessToken']
                response.success()
            except Exception as error:
                response.failure(str(error))
                WINDOW['preparationError']=f'{self.seller}: {error}'
                save_window()
                self.environment.process_exit_code=2
                gevent.spawn(self.environment.runner.quit)
                raise StopUser()
        # A ready user has credentials, queue ownership, and a validated token.
        READY_USERS.append({'sellerId':self.seller,'readyUnixNs':time.time_ns(),'tokenExpiresUnix':expiry,'products':len(QUEUES[self.seller])})
        if len(READY_USERS)==EXPECTED_USERS:
            WINDOW['ready']=READY_USERS
            WINDOW['readyUsers']=len(READY_USERS)
            WINDOW['preparationSeconds']=(time.time_ns()-WINDOW['preparationStartedUnixNs'])/1e9
            self.environment.runner.stats.reset_all()
            START=time.monotonic()
            WINDOW['startUnixNs']=time.time_ns()
            WINDOW['deadlineUnixNs']=WINDOW['startUnixNs']+int(DURATION*1e9)
            save_window()
            READY.set()
            gevent.spawn(stop_after_deadline,self.environment)
        READY.wait()

    @task
    def confirm(self):
        if time.monotonic()>=START+DURATION:
            raise StopUser()
        product=next(self.products,None)
        if product is None:
            EXHAUSTED_USERS.add(self.seller)
            if len(EXHAUSTED_USERS)==EXPECTED_USERS:
                WINDOW['poolExhaustedUnixNs']=time.time_ns()
                request_stop(self.environment,'pool_exhausted')
            raise StopUser()
        body={'buyerId':product['buyerId'],'priceCents':product['priceCents'],'idempotencyKey':product['idempotencyKey']}
        for attempt in range(1,RETRIES+2):
            launched=time.monotonic()
            launched_unix=time.time_ns()
            if launched>=START+DURATION:
                return
            with self.client.post('/products/'+product['productId']+'/sale',json=body,name='/products/:id/sale',timeout=(min(5,TIMEOUT),TIMEOUT),catch_response=True) as response:
                received=time.monotonic()
                received_unix=time.time_ns()
                try:
                    payload=response.json()
                except Exception:
                    payload=None
                status=response.status_code or 0
                valid=status==200 and isinstance(payload,dict) and payload.get('success') is True and payload.get('code')=='sold' and all(payload.get(field)==product[field] for field in ['productId','sellerId','buyerId','priceCents','idempotencyKey'])
                if valid:
                    response.success()
                else:
                    response.failure(f'status={status} code={(payload or {}).get("code","invalid response")}')
                record={'productId':product['productId'],'sellerId':self.seller,'attempt':attempt,'startedOffset':launched-START,'receivedOffset':received-START,'startedUnixNs':launched_unix,'receivedUnixNs':received_unix,'elapsedMs':(received-launched)*1000,'status':status,'response':payload,'error':str(getattr(response,'error','') or '')}
                WRITER.write(json.dumps(record,separators=(',',':'))+'\n')
            if valid or status==409 or (status!=0 and status<500):
                return
            # Immediate same-key, identical-content retry remains serial on VU.
