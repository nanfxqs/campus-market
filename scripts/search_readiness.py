#!/usr/bin/env python3
"""Bounded full V2 Search-cohort preparation, outside any measured window."""
import argparse,json,pathlib,time,urllib.request
from datetime import datetime,timezone
EXPECTED_TOTAL=16667

def wait_for_full_cohort(api,output,observation_format,timeout=180,interval=1):
    deadline=time.monotonic()+timeout
    with pathlib.Path(output).open('w') as observations:
        while True:
            remaining=deadline-time.monotonic()
            if remaining<=0:raise TimeoutError('Search full-cohort ingestion did not become ready within bounded preparation')
            record={'UTC':datetime.now(timezone.utc).isoformat()} if observation_format=='sale' else {'unixNs':time.time_ns()}
            ready=False
            try:
                with urllib.request.urlopen(api+'/search?q=%E5%8D%95%E8%BD%A6',timeout=min(15,remaining)) as response:result=json.load(response)
                ready=result.get('total')==EXPECTED_TOTAL
                record.update({'expectedTotal':EXPECTED_TOTAL,'actualTotal':result.get('total'),'ready':ready} if observation_format=='sale' else {'expected':EXPECTED_TOTAL,'actual':result.get('total'),'ready':ready})
            except Exception as error:record['error']=str(error)
            observations.write(json.dumps(record)+'\n');observations.flush()
            if ready:return
            remaining=deadline-time.monotonic()
            if remaining<=0:raise TimeoutError('Search full-cohort ingestion did not become ready within bounded preparation')
            time.sleep(min(interval,remaining))

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--api',required=True);parser.add_argument('--output',required=True);parser.add_argument('--format',choices=['sale','detail'],required=True)
    parser.add_argument('--timeout',type=float,default=180);parser.add_argument('--interval',type=float,default=1)
    args=parser.parse_args()
    if args.timeout<=0 or args.interval<=0:parser.error('positive timeout and interval required')
    try:wait_for_full_cohort(args.api,args.output,args.format,args.timeout,args.interval)
    except TimeoutError as error:parser.exit(1,str(error)+'\n')
if __name__=='__main__':main()
