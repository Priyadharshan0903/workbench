"""Integration smoke test: inspect and stop only an isolated HTTP process created by this test."""
from pathlib import Path
import re
import select
import subprocess
import sys
import tempfile
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import inventory
import server


with tempfile.TemporaryDirectory(prefix='workbench-owned-test-') as folder:
    child = subprocess.Popen([sys.executable,'-u','-m','http.server','0','--bind','127.0.0.1'],
                             cwd=folder, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        if not select.select([child.stdout],[],[],5)[0]:
            raise RuntimeError('Isolated HTTP test server did not start.')
        line = child.stdout.readline()
        match = re.search(r'port (\d+)',line)
        if not match:
            raise RuntimeError('Isolated HTTP test server failed: '+line)
        project = {'id':'isolated-test','name':'Owned test service','description':'Test',
                   'kind':'web','path':folder,'ports':[int(match[1])],'lifecycle':'active'}
        with patch.object(inventory,'read_registry',return_value={'revision':1,'projects':[project]}):
            snapshot = server.state()
            service = next(s for s in snapshot['services'] if s['pid']==child.pid)
            assert service['projectId']=='isolated-test'
            if service.get('stopUnavailable'):
                print('SKIPPED live stop: ' + service['stopUnavailable'])
                raise SystemExit(0)
            assert service['canStop'] and service['stopToken']
            server.stop_service(child.pid,service['stopToken'])
            child.wait(timeout=5)
            assert child.returncode is not None
        print('Live integration passed: owned service discovered by directory, identity verified, graceful stop received.')
    finally:
        if child.poll() is None:
            child.terminate()
            child.wait(timeout=5)
        child.stdout.close()
