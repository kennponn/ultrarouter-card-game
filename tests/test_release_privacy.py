from pathlib import Path
import hashlib
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
import zipfile


PROJECT = Path(__file__).resolve().parents[1]


class ReleasePrivacyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.root = self.directory/'project'
        self.root.mkdir()
        (self.root/'release-review.json').write_text('{"files": {}}', encoding='utf-8')
        spec = importlib.util.spec_from_file_location('privacy_audit', PROJECT/'scripts/audit-release.py')
        self.audit = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.audit)
        self.audit.ROOT = self.root
        self.markers = self.directory/'private-markers.json'
        self.markers.write_text(json.dumps(['internal.example']), encoding='utf-8')

    def write(self, relative, content):
        path = self.root/relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content)
        return path

    def scan(self):
        files = self.audit.release_files()
        markers = self.audit.load_private_markers(self.markers, files)
        return self.audit.scan(files, markers)

    def test_same_origin_and_localhost_settings_pass(self):
        self.write('js/config.js', b'const apiBase = ""; const home = "http://localhost:8080";')
        findings, _ = self.scan()
        self.assertEqual(findings, [])

    def test_site_domain_blocks_release_without_printing_its_value(self):
        self.write('js/config.js', b'const apiBase = "https://INTERNAL.EXAMPLE/api";')
        findings, _ = self.scan()
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0]['category'], 'private_marker')
        self.assertNotIn('internal.example', json.dumps(findings).lower())

    def test_binary_and_utf16_metadata_are_checked(self):
        for encoding in ('utf-8', 'utf-16-le', 'utf-16-be'):
            with self.subTest(encoding=encoding):
                self.write('assets/portrait.webp', b'RIFF\xff\x00'+('INTERNAL.EXAMPLE').encode(encoding))
                findings, _ = self.scan()
                self.assertEqual([item['category'] for item in findings], ['private_marker'])

    def test_marker_in_filename_is_blocked_and_redacted(self):
        self.write('assets/internal.example.webp', b'RIFF')
        findings, _ = self.scan()
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0]['path'], 'assets/[private marker].webp')

    def test_windows_path_marker_matches_forward_slash_source(self):
        self.markers.write_text(json.dumps(['C:\\private\\deploy']), encoding='utf-8')
        self.write('js/config.js', b'workspace = "C:/private/deploy"')
        findings, _ = self.scan()
        self.assertEqual([item['category'] for item in findings], ['private_marker'])

    def test_credential_review_cannot_allow_a_site_marker(self):
        path = self.write('backend/model/gacha_game_test.go', b'// internal.example')
        (self.root/'release-review.json').write_text(json.dumps({'files': {
            'backend/model/gacha_game_test.go': {
                'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                'categories': ['private_marker'],
            },
        }}), encoding='utf-8')
        findings, _ = self.scan()
        self.assertEqual([item['category'] for item in findings], ['private_marker'])

    def test_private_marker_configuration_must_not_be_published(self):
        published = self.write('SOURCE.lock.json', self.markers.read_bytes())
        with self.assertRaisesRegex(RuntimeError, 'must stay outside'):
            self.audit.load_private_markers(published, self.audit.release_files())

    def test_local_private_configuration_is_excluded(self):
        self.markers = self.write('.local/private-markers.json', self.markers.read_bytes())
        files = self.audit.release_files()
        self.assertNotIn(self.markers, files)
        self.assertEqual(self.audit.load_private_markers(self.markers, files), ('internal.example',))

    def test_missing_empty_or_invalid_marker_configuration_fails_closed(self):
        for content in ('[]', '[""]', '[42]', '{"markers": []}', 'not JSON'):
            with self.subTest(content=content):
                self.markers.write_text(content, encoding='utf-8')
                with self.assertRaises(RuntimeError):
                    self.audit.load_private_markers(self.markers, self.audit.release_files())
        with self.assertRaises(RuntimeError):
            self.audit.load_private_markers(self.directory/'missing.json', self.audit.release_files())

    def test_packager_rejects_an_endpoint_before_creating_an_archive(self):
        for name in ('audit-release.py', 'package-release.py'):
            target = self.root/'scripts'/name
            target.parent.mkdir(exist_ok=True)
            shutil.copyfile(PROJECT/'scripts'/name, target)
        self.write('js/config.js', b'const apiBase = "https://internal.example/api";')
        result = subprocess.run([
            sys.executable, str(self.root/'scripts/package-release.py'),
            '--private-markers', str(self.markers),
        ], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('internal.example', (result.stdout+result.stderr).lower())
        self.assertEqual(list(self.root.rglob('*.zip')), [])

    def test_full_host_backend_and_deployment_files_are_rejected(self):
        for relative in ('backend/web/package.json', 'Dockerfile', 'scripts/dev.py'):
            with self.subTest(path=relative):
                target = self.write(relative, b'# full-host source must not be published')
                with self.assertRaisesRegex(RuntimeError, 'not card-game source'):
                    self.audit.release_files()
                target.unlink()

    def test_content_release_allowlist_accepts_units_and_rejects_unrelated_files(self):
        for relative in ('content/README.md', 'content/cards/index.json', 'content/cards/c01/card.json', 'content/enemies/wolf/enemy.json', 'content/bosses/abyss-lord/boss.json', 'content/bosses/abyss-lord/assets/portrait.webp'):
            target = self.write(relative, b'{}')
            self.assertIn(target, self.audit.release_files())
        for relative in ('content/config.json', 'content/cards/c01/accounts.json', 'content/bosses/abyss-lord/assets/private.env'):
            target = self.write(relative, b'{}')
            with self.assertRaisesRegex(RuntimeError, 'not card-game content'):
                self.audit.release_files()
            target.unlink()

    def test_packager_blocks_a_complete_host_backend(self):
        for name in ('audit-release.py', 'package-release.py'):
            target = self.root/'scripts'/name
            target.parent.mkdir(exist_ok=True)
            shutil.copyfile(PROJECT/'scripts'/name, target)
        self.write('backend/web/package.json', b'{"name": "newapi-web"}')
        result = subprocess.run([
            sys.executable, str(self.root/'scripts/package-release.py'),
        ], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(list(self.root.rglob('*.zip')), [])

    def test_clean_package_excludes_secrets_and_has_verified_file_hashes(self):
        for name in ('audit-release.py', 'package-release.py'):
            target = self.root/'scripts'/name
            target.parent.mkdir(exist_ok=True)
            shutil.copyfile(PROJECT/'scripts'/name, target)
        self.write('js/config.js', b'const apiBase = "";')
        self.write('.env', b'SESSION_SECRET=not-for-publication')
        self.write('.local/data/game.db', b'private database')
        self.write('.local/release-private-markers.json', self.markers.read_bytes())
        result = subprocess.run([
            sys.executable, str(self.root/'scripts/package-release.py'),
        ], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        archives = list((self.root/'.local/releases').glob('*.zip'))
        self.assertEqual(len(archives), 1)
        with zipfile.ZipFile(archives[0]) as archive:
            prefix = 'starfrost-game/'
            manifest = json.loads(archive.read(prefix+'FILE-MANIFEST.json'))
            self.assertEqual(len(archive.namelist()), len(manifest)+1)
            self.assertFalse(any('.local/' in name or name.endswith('.env') for name in archive.namelist()))
            for entry in manifest:
                content = archive.read(prefix+entry['path'])
                self.assertEqual(hashlib.sha256(content).hexdigest(), entry['sha256'])
                self.assertEqual(len(content), entry['bytes'])

if __name__ == '__main__':
    unittest.main()
