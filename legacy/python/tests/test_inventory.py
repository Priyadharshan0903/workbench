import copy
import http.client
from http.server import ThreadingHTTPServer
import json
from pathlib import Path
import signal
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import inventory
import server
sys.path.insert(0, str(Path(__file__).resolve().parents[3] / 'scripts'))
import install_agent_instructions


def project(**values):
    return dict({'id': 'sample', 'name': 'Sample', 'description': 'A test tool', 'kind': 'web',
                 'path': '/tmp/sample', 'url': 'http://127.0.0.1:3000/', 'ports': [3000],
                 'lifecycle': 'active', 'owner': 'Tester', 'launch': 'python3 server.py',
                 'maintenance': '', 'docs': 'README.md'}, **values)


class RegistryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.path = self.root / 'registry.json'
        self.original = {'version': 1, 'revision': 1, 'projects': [project()]}
        self.path.write_text(json.dumps(self.original))

    def tearDown(self):
        self.temp.cleanup()

    def test_save_preserves_previous_and_updates_one_record(self):
        result = inventory.save_project(project(name='Updated'), 1, self.path)
        self.assertEqual(result['revision'], 2)
        self.assertEqual(len(result['projects']), 1)
        self.assertEqual(inventory.read_registry(self.path)['projects'][0]['name'], 'Updated')
        self.assertEqual(json.loads((self.root / 'registry.previous.json').read_text()), self.original)

    def test_stale_update_does_not_replace_data(self):
        inventory.save_project(project(name='New'), 1, self.path)
        with self.assertRaisesRegex(ValueError, 'registry changed'):
            inventory.save_project(project(name='Stale'), 1, self.path)
        self.assertEqual(inventory.read_registry(self.path)['projects'][0]['name'], 'New')

    def test_invalid_updates_do_not_write(self):
        for fields in [{'url':'javascript:alert(1)'}, {'ports':[True]}, {'ports':[65536]},
                       {'ports':[3000,3000]}, {'path':'relative/path'}, {'id':[]},
                       {'kind':[]}, {'lifecycle':'deleted'}, {'path':'/tmp/.env'},
                       {'name':''}, {'url':'http://user:secret@localhost/'}]:
            with self.subTest(fields=fields), self.assertRaises(ValueError):
                inventory.save_project(project(**fields), 1, self.path)
        self.assertEqual(inventory.read_registry(self.path), self.original)

    def test_corrupt_registry_is_not_reset(self):
        self.path.write_text('{')
        with self.assertRaises(ValueError):
            inventory.save_project(project(), path=self.path)
        self.assertEqual(self.path.read_text(), '{')

    def test_global_instruction_install_is_idempotent_and_preserves_existing(self):
        original = '# Existing instructions\nKeep this rule.\n'
        once = install_agent_instructions.updated(original)
        self.assertIn('Keep this rule.', once)
        self.assertEqual(once, install_agent_instructions.updated(once))
        with self.assertRaises(ValueError):
            install_agent_instructions.updated(install_agent_instructions.START)


class SkillTests(unittest.TestCase):
    def test_claude_and_codex_linked_skills_combine_and_preserve_providers(self):
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            skill = home / '.agents/skills/design'
            skill.mkdir(parents=True)
            (skill / 'SKILL.md').write_text('---\nname: design\ndescription: >\n  A useful skill\n  for design.\n---\nBody')
            claude = home / '.claude/skills'
            claude.mkdir(parents=True)
            (claude / 'design').symlink_to(skill, target_is_directory=True)
            (skill / 'cycle').symlink_to(claude, target_is_directory=True)
            only = claude / 'claude-only'
            only.mkdir()
            (only / 'SKILL.md').write_text('---\nname: Claude only\ndescription: Only here\n---')
            result = inventory.discover_skills([], home)
            self.assertEqual(len(result['skills']), 2)
            linked = next(s for s in result['skills'] if s['name'] == 'design')
            self.assertEqual(set(linked['providers']), {'codex', 'claude'})
            self.assertEqual(len(linked['locations']), 2)
            self.assertEqual(linked['description'], 'A useful skill for design.')

    def test_project_and_synced_claude_skills_are_found(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            local = root / 'project/.claude/skills/custom'
            synced = root / '.claude/plugins/synced/plugin/skills/special'
            for folder in [local, synced]:
                folder.mkdir(parents=True)
                (folder / 'SKILL.md').write_text('No metadata')
            result = inventory.discover_skills([project(path=str(root / 'project'))], root)
            self.assertEqual({s['name'] for s in result['skills']}, {'custom', 'special'})
            self.assertTrue(all(s['providers'] == ['claude'] for s in result['skills']))

    def test_missing_roots_and_cycles_are_safe(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            cycle = root / '.claude/skills'
            cycle.mkdir(parents=True)
            (cycle / 'again').symlink_to(cycle, target_is_directory=True)
            self.assertEqual(inventory.discover_skills([], root)['skills'], [])


class ProcessTests(unittest.TestCase):
    @patch.object(inventory, 'process_cwds', return_value={123:'/tmp/shared',124:'/tmp/shared'})
    @patch.object(inventory, 'run', return_value='p123\ncNode\nn127.0.0.1:3000\np124\ncNode\nn127.0.0.1:3001\n')
    def test_same_source_root_prefers_expected_port(self, *_):
        result=inventory.inspect_services([project(path='/tmp/shared'),project(id='website',name='Website',path='/tmp/shared',ports=[3001])])
        self.assertEqual([s['projectId'] for s in result],['sample','website'])

    @patch.object(inventory, 'inspect_services', return_value=[])
    @patch.object(inventory, 'read_registry', return_value={'revision':1,'projects':[project(path='/workbench-test-missing-project')]})
    def test_missing_folder_is_not_a_normal_stopped_project(self, *_):
        self.assertEqual(inventory.snapshot()['projects'][0]['status'],'unlocated')

    def test_ipv4_ipv6_and_duplicate_descriptors(self):
        output = 'p123\ncPython\nf1\nn127.0.0.1:3000\nf2\nn[::1]:3000\nf3\nn[::1]:3000\nf4\nn*:3001\n'
        result = inventory.parse_listeners(output)
        self.assertEqual(result[0]['ports'], [3000,3001])
        self.assertEqual(len(result[0]['endpoints']), 3)

    @patch.object(inventory, 'process_cwds', return_value={123:'/tmp/sample', 124:'/tmp/sample/sub', 125:'/other'})
    @patch.object(inventory, 'run', return_value='p123\ncPython\nn127.0.0.1:3000\np124\ncNode\nn127.0.0.1:3001\np125\ncOther\nn*:4000\n')
    def test_project_matching_uses_path_not_expected_port(self, *_):
        result = inventory.inspect_services([project(),project(id='nested',name='Nested',path='/tmp/sample/sub',ports=[4000])])
        self.assertEqual([p['projectId'] for p in result], ['sample','nested',None])
        self.assertEqual(result[1]['unexpectedPorts'], [3001])
        self.assertFalse(result[2]['canStop'])

    def test_path_prefix_cannot_match_another_folder(self):
        self.assertFalse(inventory.within('/tmp/sample-evil', '/tmp/sample'))
        self.assertTrue(inventory.within('/tmp/sample/sub', '/tmp/sample'))

    @patch.object(inventory, 'inspect_services', side_effect=OSError('Permission denied'))
    @patch.object(inventory, 'read_registry', return_value={'revision':1,'projects':[project()]})
    def test_inspection_failure_is_unknown_not_stopped(self, *_):
        result = inventory.snapshot()
        self.assertEqual(result['projects'][0]['status'], 'unknown')
        self.assertIn('Permission denied', result['inspectionError'])

    @patch.object(inventory, 'inspect_services', return_value=[{'pid':123,'projectId':'sample','ports':[3000],'unexpectedPorts':[]},{'pid':124,'projectId':'sample','ports':[3001],'unexpectedPorts':[3001]},{'pid':125,'projectId':None,'ports':[3000],'unexpectedPorts':[]}])
    @patch.object(inventory, 'read_registry', return_value={'revision':1,'projects':[project()]})
    def test_conflicts_and_extra_processes_flagged(self, *_):
        result = inventory.snapshot()['projects'][0]
        self.assertTrue(result['extraServices'])
        self.assertEqual(result['status'], 'conflict')
        self.assertEqual(result['conflicts'][0]['pid'], 125)


class StopTests(unittest.TestCase):
    def setUp(self):
        self.service = {'pid':123,'name':'Python','canStop':True,'cwd':'/tmp/sample','endpoints':['127.0.0.1:3000']}

    @patch.object(server, 'stop_token', side_effect=PermissionError('Identity check blocked'))
    def test_state_explains_disabled_controls_when_identity_is_inaccessible(self, _):
        with patch.object(inventory, 'snapshot', return_value={'services':[self.service],'inspectionError':None}):
            result = server.state()
        self.assertFalse(result['services'][0]['canStop'])
        self.assertIn('Identity check blocked', result['services'][0]['stopUnavailable'])
        self.assertTrue(result['controlWarnings'])

    @patch.object(server.os, 'kill')
    @patch.object(server, 'stop_token', return_value='new-identity')
    def test_changed_identity_does_not_signal(self, _, kill):
        with patch.object(inventory, 'snapshot', return_value={'services':[self.service],'inspectionError':None}):
            with self.assertRaisesRegex(ValueError, 'process changed'):
                server.stop_service(123, 'old-identity')
        kill.assert_not_called()

    @patch.object(server.os, 'kill')
    def test_unknown_or_protected_process_is_never_signalled(self, kill):
        for services in [[], [dict(self.service,canStop=False)]]:
            with patch.object(inventory, 'snapshot', return_value={'services':services,'inspectionError':None}):
                with self.assertRaises(ValueError):
                    server.stop_service(123, 'token')
        kill.assert_not_called()

    @patch.object(server.os, 'kill')
    @patch.object(server, 'stop_token', return_value='verified')
    def test_explicit_verified_request_only_sends_sigterm_to_pid(self, _, kill):
        with patch.object(inventory, 'snapshot', return_value={'services':[self.service],'inspectionError':None}):
            self.assertTrue(server.stop_service(123,'verified')['ok'])
        kill.assert_called_once_with(123, signal.SIGTERM)


class HTTPTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.http = ThreadingHTTPServer(('127.0.0.1',0), server.Handler)
        cls.thread = threading.Thread(target=cls.http.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.http.shutdown()
        cls.http.server_close()
        cls.thread.join()

    def request(self, method, path, body=None, headers=None):
        connection = http.client.HTTPConnection('127.0.0.1',self.http.server_port,timeout=3)
        connection.request(method,path,body,headers or {})
        response = connection.getresponse()
        status, output = response.status, response.read()
        connection.close()
        return status, output

    def test_health_and_fixed_public_files_only(self):
        for path in ['/api/health','/','/app.js','/styles.css','/favicon.svg']:
            self.assertEqual(self.request('GET',path)[0],200)
        for path in ['/registry.json','/../registry.json','/.env','/server.py']:
            self.assertEqual(self.request('GET',path)[0],404)

    def test_host_origin_and_cross_site_rejected(self):
        for headers in [{'Host':'evil.test'}, {'Origin':'https://evil.test'}, {'Sec-Fetch-Site':'cross-site'}]:
            self.assertEqual(self.request('GET','/api/health',headers=headers)[0],403)

    def test_writes_require_csrf_even_from_loopback(self):
        with patch.object(inventory, 'save_project') as save:
            self.assertEqual(self.request('POST','/api/project','{}',{'Content-Type':'application/json'})[0],403)
            save.assert_not_called()

    def test_invalid_json_and_oversized_requests_rejected(self):
        headers = {'Content-Type':'application/json','X-Workbench-Token':server.CSRF}
        self.assertEqual(self.request('POST','/api/project','{',headers)[0],400)
        self.assertEqual(self.request('POST','/api/project','[]',headers)[0],400)
        self.assertEqual(self.request('POST','/api/project','x'*32769,headers)[0],400)

    def test_post_saves_validated_record_with_revision(self):
        headers = {'Content-Type':'application/json','X-Workbench-Token':server.CSRF}
        payload = {'project':project(),'revision':1}
        with patch.object(inventory, 'save_project') as save:
            self.assertEqual(self.request('POST','/api/project',json.dumps(payload),headers)[0],200)
            save.assert_called_once_with(payload['project'],1)


if __name__ == '__main__':
    unittest.main()
