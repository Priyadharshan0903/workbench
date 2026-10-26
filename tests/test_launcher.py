import copy
import json
from pathlib import Path
import shlex
import socket
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch
from urllib.request import build_opener, ProxyHandler

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import discovery
import inventory
import supervisor
import server
from test_inventory import project


class DiscoveryTests(unittest.TestCase):
    def test_discovery_suggests_projects_and_skips_private_dependency_and_linked_trees(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            app=root/'website';app.mkdir()
            (app/'package.json').write_text(json.dumps({'name':'my-site','scripts':{'dev':'vite --port 8833'},'description':'My site'}))
            for folder in [root/'node_modules/hidden',root/'.ssh/hidden',root/'.env.private/hidden']:
                folder.mkdir(parents=True);(folder/'package.json').write_text('{"scripts":{"dev":"vite"}}')
            (root/'linked').symlink_to(app,target_is_directory=True)
            python=root/'python-app';python.mkdir();(python/'start.py').write_text('URL="http://127.0.0.1:8834"\n# --no-browser')
            (python/'README.md').write_text('docs')
            with patch.object(subprocess,'Popen',side_effect=AssertionError('Discovery must never execute project code')):
                data=discovery.discover(str(root))
            self.assertEqual(len(data['candidates']),2)
            site=next(c for c in data['candidates'] if c['project']['id']=='website')['project']
            self.assertEqual(site['ports'],[8833]);self.assertEqual(site['launch'],'npm run dev')
            py=next(c for c in data['candidates'] if c['project']['id']=='python-app')['project']
            self.assertIn('--no-browser',py['launch']);self.assertEqual(py['docs'],'README.md')
            self.assertEqual(len(discovery.discover(str(root),[site])['candidates']),1)

    def test_static_port_avoids_catalog_reservations(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);(root/'index.html').write_text('<title>My static page</title>')
            data=discovery.discover(str(root),[project(ports=[8800,8801])])
            self.assertEqual(data['candidates'][0]['project']['ports'],[8802])

    def test_system_and_secret_folders_are_rejected(self):
        for path in ['/',str(Path.home())]:
            with self.assertRaises(ValueError):discovery.discover(path)
        with tempfile.TemporaryDirectory() as temp:
            path=Path(temp)/'.env';path.mkdir()
            with self.assertRaises(ValueError):discovery.discover(str(path))


class PlanTests(unittest.TestCase):
    def test_commands_are_literal_argv_and_sequential_steps(self):
        commands=supervisor.parse_commands("python3 -c 'print(1); print(2)' && python3 server.py",'/tmp')
        self.assertEqual(len(commands),2);self.assertEqual(commands[0][2],'print(1); print(2)')
        self.assertEqual(commands[0][0],sys.executable)

    def test_shell_substitution_redirection_and_backgrounding_rejected(self):
        for command in ['python3 server.py &','python3 server.py > out','python3 $(whoami)','python3 `whoami`','python3 server.py | cat','python3 server.py; echo done']:
            with self.subTest(command=command),self.assertRaises(ValueError):supervisor.parse_commands(command,'/tmp')

    def test_missing_runtime_has_actionable_error(self):
        with self.assertRaisesRegex(ValueError,'not installed'):
            supervisor.parse_commands('workbench-nonexistent-runtime server.py','/tmp')

    def test_changed_plan_and_port_reservation_prevent_execution(self):
        with tempfile.TemporaryDirectory() as temp:
            entry=project(path=temp)
            registry={'revision':4,'projects':[entry]}
            with patch.object(inventory,'read_registry',return_value=registry),patch.object(subprocess,'Popen') as popen:
                with self.assertRaisesRegex(ValueError,'changed'):
                    supervisor.start_project('sample','old-fingerprint',4)
                registry['projects'].append(project(id='other',name='Other',path='/other'))
                with self.assertRaisesRegex(ValueError,'reserved'):
                    supervisor.launch_plan('sample')
                popen.assert_not_called()

    def test_paused_and_self_projects_do_not_launch(self):
        for entry in [project(lifecycle='paused'),project(id='workbench')]:
            with patch.object(inventory,'read_registry',return_value={'revision':1,'projects':[entry]}):
                with self.assertRaises(ValueError):supervisor.launch_plan(entry['id'])


class ReadinessTests(unittest.TestCase):
    def test_responding_unmatched_listener_remains_unverified(self):
        p=project(status='conflict',services=[],conflicts=[{'projectId':None}],extraServices=False)
        with patch.object(supervisor,'check_http',return_value={'state':'ready','message':'HTTP 200'}),patch.object(supervisor,'read_run',return_value=None):
            result=supervisor.enrich({'projects':[p]})['projects'][0]
        self.assertEqual(result['status'],'unverified')

    def test_inspection_failure_never_becomes_stopped_or_ready(self):
        p=project(status='unknown',services=[],conflicts=[],extraServices=False)
        with patch.object(supervisor,'read_run',return_value=None):
            result=supervisor.enrich({'projects':[p]})['projects'][0]
        self.assertEqual(result['status'],'unknown')

    def test_listener_with_http_failure_is_not_ready(self):
        p=project(status='running',services=[{}],conflicts=[],extraServices=False)
        with patch.object(supervisor,'check_http',return_value={'state':'error','message':'HTTP 404'}),patch.object(supervisor,'read_run',return_value=None):
            result=supervisor.enrich({'projects':[p]})['projects'][0]
        self.assertEqual(result['status'],'unhealthy')

    def test_log_redaction_handles_ansi_and_common_credentials(self):
        text=supervisor.redact('\x1b[31mERROR\x1b[0m token=private-value Bearer abcd.efg ghp_abcdefghijklmnopq')
        self.assertNotIn('private-value',text);self.assertNotIn('abcd.efg',text);self.assertNotIn('ghp_',text);self.assertIn('ERROR',text)

    def test_failed_launch_diagnosis_names_the_next_step(self):
        self.assertIn('dependency',supervisor.diagnose('ModuleNotFoundError: no module named demo',1))
        self.assertIn('busy port',supervisor.diagnose('EADDRINUSE 3000',1))
        self.assertIn('runtime version',supervisor.diagnose('npm EBADENGINE',1))
        self.assertIn('missing file',supervisor.diagnose('ENOENT package.json',1))

    def test_start_requires_actual_integer_revision(self):
        with self.assertRaisesRegex(ValueError,'Review'):
            supervisor.start_project('sample','fingerprint',True)


class RunnerIntegrationTests(unittest.TestCase):
    def test_launch_duplicate_guard_readiness_logs_and_exit(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            with socket.socket() as probe:
                probe.bind(('127.0.0.1',0));port=probe.getsockname()[1]
            fixture=root/'site.py'
            fixture.write_text('''from http.server import BaseHTTPRequestHandler,HTTPServer
import threading
class H(BaseHTTPRequestHandler):
 def do_GET(self):
  self.send_response(200);self.end_headers();self.wfile.write(b"fixture ready")
  if self.path=="/__stop":threading.Thread(target=self.server.shutdown).start()
http=HTTPServer(("127.0.0.1",PORT),H)
print("fixture started",flush=True)
http.serve_forever()
http.server_close()
'''.replace('PORT',str(port)))
            entry=project(path=str(root),url=f'http://127.0.0.1:{port}/',ports=[port],launch='python3 -c '+shlex.quote('print("prepare complete")')+' && python3 site.py')
            registry={'version':1,'revision':1,'projects':[entry]}
            registry_file=root/'registry.json';registry_file.write_text(json.dumps(registry))
            original_read=inventory.read_registry
            opener=build_opener(ProxyHandler({}))
            with patch.object(inventory,'read_registry',side_effect=lambda *_args:original_read(registry_file)),patch.object(supervisor,'RUNTIME',root/'.runtime'):
                try:
                    plan=supervisor.launch_plan('sample');first=supervisor.start_project('sample',plan['fingerprint'],1)
                    for _ in range(80):
                        state=supervisor.enrich(inventory.snapshot())
                        if state['projects'][0]['status']=='ready':break
                        time.sleep(.05)
                    self.assertEqual(state['projects'][0]['status'],'ready')
                    second=supervisor.start_project('sample',plan['fingerprint'],1)
                    self.assertTrue(second['alreadyRunning']);self.assertEqual(supervisor.read_run('sample')['runId'],first['runId'])
                    logs=supervisor.logs('sample')['text'];self.assertIn('prepare complete',logs);self.assertIn('fixture started',logs)
                    with opener.open(entry['url']+'__stop',timeout=2) as response:self.assertEqual(response.status,200)
                    process=supervisor.CHILDREN[first['runId']];self.assertEqual(process.wait(timeout=5),0)
                    self.assertEqual(supervisor.read_run('sample')['phase'],'completed')
                    entry['launch']='python3 -c '+shlex.quote('print("ModuleNotFoundError: fixture dependency"); raise SystemExit(7)')
                    registry['revision']=2;registry_file.write_text(json.dumps(registry))
                    plan=supervisor.launch_plan('sample');failed=supervisor.start_project('sample',plan['fingerprint'],2)
                    supervisor.CHILDREN[failed['runId']].wait(timeout=5)
                    failed_state=supervisor.enrich(inventory.snapshot())['projects'][0]
                    self.assertEqual(failed_state['status'],'failed')
                    self.assertIn('dependency',failed_state['statusMessage'])
                finally:
                    try:opener.open(entry['url']+'__stop',timeout=1).close()
                    except OSError:pass


if __name__=='__main__':unittest.main()
