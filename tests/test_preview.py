from pathlib import Path
import hashlib
from http.client import HTTPConnection
from http.server import ThreadingHTTPServer
import importlib.util
import json
import tempfile
import threading
import unittest


PROJECT = Path(__file__).resolve().parents[1]


class GamePreviewTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        for name, content in {
            'index.html': b'<main>game preview</main>',
            'style.css': b'body { color: white; }',
            'mobile.css': b'body { margin: 0; }',
            'harbor.css': b'.harbor-home {}',
            'voyage.css': b'.voyage-map {}',
            'fleet.css': b'.fleet-board {}',
            'js/config.js': b'const apiBase = "";',
            'assets/portrait.webp': b'RIFF',
            'content/cards/c01/assets/portrait.webp': b'RIFF-card',
            'content/enemies/wolf/assets/portrait-cut.webp': b'RIFF-enemy',
            'content/bosses/abyss-lord/assets/portrait.webp': b'RIFF-boss',
            'content/cards/c01/card.json': b'{"maintenance": true}',
            '.env': b'private configuration',
            'backend/model/gacha.go': b'private source, not a browser asset',
            'scripts/preview.py': b'private development script',
            '.local/data/game.db': b'private database',
        }.items():
            path = self.root/name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        spec = importlib.util.spec_from_file_location('game_preview', PROJECT/'scripts/preview.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self.server = ThreadingHTTPServer(('127.0.0.1', 0), module.preview_handler(self.root))
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_server)

    def stop_server(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def request(self, path, method='GET', body=None, headers=None):
        connection = HTTPConnection('127.0.0.1', self.server.server_port, timeout=5)
        try:
            connection.request(method, path, body=body, headers=headers or {})
            response = connection.getresponse()
            return response.status, dict(response.getheaders()), response.read()
        finally:
            connection.close()

    def test_game_frontend_and_artwork_are_served(self):
        for path in ('/gacha/', '/gacha/style.css', '/gacha/mobile.css', '/gacha/harbor.css', '/gacha/voyage.css', '/gacha/fleet.css', '/gacha/js/config.js', '/gacha/assets/portrait.webp', '/gacha/content/cards/c01/assets/portrait.webp', '/gacha/content/enemies/wolf/assets/portrait-cut.webp', '/gacha/content/bosses/abyss-lord/assets/portrait.webp'):
            with self.subTest(path=path):
                status, headers, content = self.request(path)
                self.assertEqual(status, 200)
                self.assertEqual(headers['X-Content-Type-Options'], 'nosniff')
                self.assertTrue(content)

    def test_preview_login_and_gameplay_fail_closed_without_writing_files(self):
        before = {path: hashlib.sha256(path.read_bytes()).hexdigest() for path in self.root.rglob('*') if path.is_file()}
        for path in ('/api/user/auth/refresh', '/api/user/gacha/profile', '/api/user/gacha/run/finish', '/api/user/gacha/action'):
            with self.subTest(path=path):
                status, headers, content = self.request(path, 'POST', '{"user_id":1,"stars":999,"quota":999999}')
                self.assertEqual(status, 401)
                self.assertEqual(headers['Cache-Control'], 'no-store')
                response = json.loads(content)
                self.assertFalse(response['success'])
                self.assertNotIn('access_token', response)
        after = {path: hashlib.sha256(path.read_bytes()).hexdigest() for path in self.root.rglob('*') if path.is_file()}
        self.assertEqual(after, before)

    def test_source_environment_database_and_dev_files_are_not_served(self):
        for path in ('/gacha/.env', '/gacha/backend/model/gacha.go', '/gacha/scripts/preview.py', '/gacha/.local/data/game.db', '/backend/model/gacha.go', '/gacha/content/cards/c01/card.json'):
            with self.subTest(path=path):
                status, _, content = self.request(path)
                self.assertEqual(status, 404)
                self.assertNotIn(b'private configuration', content)
                self.assertNotIn(b'private database', content)

    def test_traversal_paths_cannot_escape_the_game_asset_allowlist(self):
        for path in ('/gacha/assets/../../.env', '/gacha/assets/%2e%2e/%2e%2e/.env', '/gacha/assets/..%5c../.env', '/gacha/assets/C:%5cprivate%5c.env', '/gacha/assets/%00portrait.webp'):
            with self.subTest(path=path):
                status, _, _ = self.request(path)
                self.assertEqual(status, 404)

    def test_foreign_host_and_cross_origin_requests_are_rejected(self):
        for headers in ({'Host': 'foreign.example'}, {'Origin': 'https://foreign.example'}):
            with self.subTest(headers=headers):
                status, _, _ = self.request('/gacha/', headers=headers)
                self.assertEqual(status, 403)

    def test_same_origin_requests_are_allowed_and_static_writes_are_rejected(self):
        origin = f'http://127.0.0.1:{self.server.server_port}'
        status, _, _ = self.request('/gacha/', headers={'Origin': origin})
        self.assertEqual(status, 200)
        status, _, _ = self.request('/gacha/index.html', 'POST', 'replace the game')
        self.assertEqual(status, 405)
        self.assertEqual((self.root/'index.html').read_bytes(), b'<main>game preview</main>')


if __name__ == '__main__':
    unittest.main()
