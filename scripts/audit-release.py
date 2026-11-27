from pathlib import Path
import argparse
import hashlib
import json
import os
import re
import sys


ROOT = Path(__file__).resolve().parents[1]
ROOT_FILES = {
    'arena-ladder.css',
    'index.html', 'style.css', 'battle.css', 'battle-motion.css', 'battle-menu.css', 'mobile.css', 'harbor.css', 'voyage.css', 'voyage-map.css', 'voyage-screen.css', 'fleet.css', 'harbor-mail.css', 'package.json', 'package-lock.json',
    '.gitignore', '.gitattributes',
    'README.md', 'CONTRIBUTING.md', 'SECURITY.md', 'ASSETS.md',
    'LICENSE', 'NOTICE', 'THIRD-PARTY-LICENSES.md', 'SOURCE.lock.json',
    'release-review.json', 'VALIDATION.md', 'AGENTS.md',
}
ROOT_DIRECTORIES = {'backend', 'js', 'assets', 'content', 'tests', 'scripts', '.github'}
NON_GAME_ROOT_FILES = {'Dockerfile', 'compose.yml', 'compose.yaml', 'docker-compose.yml', 'docker-compose.yaml'}
SCOPED_FILES = {
    'backend': {
        'backend/AGENTS.md', 'backend/INTEGRATION.md',
        'backend/model/gacha.go', 'backend/model/gacha_builds.go',
        'backend/model/gacha_game.go', 'backend/model/gacha_game_battle.go',
        'backend/model/gacha_matchmaking.go', 'backend/model/gacha_modes.go',
        'backend/model/gacha_progression.go', 'backend/model/gacha_rooms.go',
        'backend/model/gacha_social.go', 'backend/model/gacha_catalog.json',
        'backend/model/gacha_ladder.go', 'backend/model/gacha_ladder_test.go', 'backend/model/gacha_contract_database_test.go',
        'backend/model/gacha_game_test.go', 'backend/model/gacha_game_battle_test.go',
        'backend/model/gacha_formation_test.go',
        'backend/model/gacha_cultivation.go', 'backend/model/gacha_cultivation.json',
        'backend/model/gacha_cultivation_test.go', 'backend/model/gacha_artifacts.go', 'backend/model/gacha_artifacts_test.go',
        'backend/model/gacha_items.go', 'backend/model/gacha_items.json',
        'backend/model/gacha_equipment.go', 'backend/model/gacha_equipment.json', 'backend/model/gacha_equipment_test.go',
        'backend/model/gacha_training.go', 'backend/model/gacha_training.json', 'backend/model/gacha_training_test.go',
        'backend/model/gacha_carry.go', 'backend/model/gacha_carry.json', 'backend/model/gacha_carry_test.go',
        'backend/model/gacha_refinement.go', 'backend/model/gacha_refinement_test.go',
        'backend/model/gacha_artifact_grades.go', 'backend/model/gacha_artifact_grades.json', 'backend/model/gacha_artifact_grades_test.go',
        'backend/model/gacha_experience.go', 'backend/model/gacha_experience_test.go',
        'backend/model/gacha_cases.go', 'backend/model/gacha_cases.json', 'backend/model/gacha_cases_test.go',
        'backend/model/gacha_vigil.go', 'backend/model/gacha_vigil.json', 'backend/model/gacha_vigil_test.go',
        'backend/model/gacha_dispatch.go', 'backend/model/gacha_dispatch_boards.go', 'backend/model/gacha_dispatch.json', 'backend/model/gacha_dispatch_test.go',
        'backend/model/gacha_cipher.go', 'backend/model/gacha_cipher_test.go',
        'backend/model/gacha_captain.go', 'backend/model/gacha_captain_test.go',
        'backend/model/gacha_voyage.go', 'backend/model/gacha_voyage_test.go',
        'backend/model/gacha_voyage_sweep.go', 'backend/model/gacha_voyage_sweep_test.go',
        'backend/model/gacha_fleet.go', 'backend/model/gacha_fleet_test.go',
        'backend/model/gacha_guild_progression.go', 'backend/model/gacha_guild_progression_test.go',
        'backend/model/gacha_guild_activity.go', 'backend/model/gacha_guild_activity_test.go',
        'backend/model/gacha_festival.go', 'backend/model/gacha_festival.json', 'backend/model/gacha_festival_test.go',
        'backend/model/gacha_mail.go', 'backend/model/gacha_mail.json', 'backend/model/gacha_mail_test.go', 'backend/model/gacha_mail_database_test.go',
        'backend/controller/gacha.go', 'backend/controller/gacha_authority_test.go',
        'backend/integration/gacha-routes.go.example', 'backend/integration/gacha-database.go.example',
        'backend/integration/gacha-ladder-worker.go.example',
    },
    'scripts': {
        'scripts/audit-release.py', 'scripts/package-release.py',
        'scripts/build-half-cpu.sh', 'scripts/preview.py', 'scripts/sync-catalog.py',
    },
    'tests': {
        'tests/server-ui-check.js', 'tests/server-authority-check.js',
        'tests/detail-contrast-check.js', 'tests/cultivation-check.js',
        'tests/refinement-check.js', 'tests/training-check.js',
        'tests/artifact-grades-check.js', 'tests/cases-check.js', 'tests/vigil-check.js', 'tests/dispatch-check.js', 'tests/cipher-check.js',
        'tests/test_release_privacy.py', 'tests/test_preview.py',
        'tests/festival-check.js', 'tests/captain-check.js', 'tests/voyage-check.js', 'tests/fleet-check.js', 'tests/mail-check.js',
        'tests/ladder-rewards-check.js', 'tests/squad-drag-check.js', 'tests/guild-growth-check.js',
        'tests/unit-catalog-check.js', 'tests/battle-formation-check.js', 'tests/battle-motion-check.js', 'tests/battle-menu-check.js',
    },
    'js': {'js/'+name+'.js' for name in ('config', 'store', 'data', 'ui', 'ladder', 'progression', 'atelier', 'carry', 'training', 'experience', 'cases', 'vigil', 'dispatch', 'cipher', 'workshop', 'battle', 'battle-motion', 'captain', 'voyage', 'voyage-map', 'fleet', 'festival', 'mail', 'main')},
    '.github': {'.github/workflows/game-checks.yml'},
}
IGNORED_PARTS = {'node_modules', 'dist', '__pycache__', '.git', '.local', '.cache'}
PRIVATE_SUFFIXES = {'.pem', '.key', '.pfx', '.p12', '.db', '.sqlite', '.sqlite3', '.dump', '.log'}
PATTERNS = {
    'private_key': re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----'),
    'aws_key': re.compile(r'\b(?:AKIA|ASIA)[A-Z0-9]{16}\b'),
    'github_token': re.compile(r'\b(?:ghp|gho|ghs|ghu|github_pat)_[A-Za-z0-9_]{20,}'),
    'api_key': re.compile(r'\bsk-[A-Za-z0-9_-]{20,}'),
    'credential_url': re.compile(r'\b(?:https?|postgres(?:ql)?|mysql|redis)://[^\s/:\'"`]+:[^\s/@\'"`]+@'),
    'quoted_secret': re.compile(
        r'(?i)\b(?:api[_-]?key|access[_-]?token|session[_-]?secret|jwt[_-]?secret|'
        r'crypto[_-]?secret|client[_-]?secret|password|passwd|authorization)\b[\s\"\']*'
        r'(?:=|:)[\s]*(?:[\"\'])([^\"\'\r\n]{8,})(?:[\"\'])'
    ),
}


def release_files():
    files = []
    for path in sorted(ROOT.rglob('*')):
        relative = path.relative_to(ROOT)
        if IGNORED_PARTS.intersection(relative.parts):
            continue
        if relative.as_posix() in NON_GAME_ROOT_FILES:
            raise RuntimeError('File is not card-game source: '+relative.as_posix())
        if relative.parts[0] not in ROOT_DIRECTORIES and relative.as_posix() not in ROOT_FILES:
            continue
        if path.is_symlink():
            raise RuntimeError('Publication does not support symbolic links: '+relative.as_posix())
        if not path.is_file():
            continue
        allowed = SCOPED_FILES.get(relative.parts[0])
        if allowed is not None and relative.as_posix() not in allowed:
            raise RuntimeError('File is not card-game source: '+relative.as_posix())
        if relative.parts[0] == 'assets' and path.suffix.lower() != '.webp' and relative.as_posix() != 'assets/PROVENANCE.json':
            raise RuntimeError('File is not card-game source: '+relative.as_posix())
        if relative.parts[0] == 'content':
            unit_source = re.fullmatch(r'content/(cards|enemies|bosses)/[a-z][a-z0-9-]*/(card|enemy|boss)\.json', relative.as_posix())
            unit_art = re.fullmatch(r'content/(cards|enemies|bosses)/[a-z][a-z0-9-]*/assets/[a-z][a-z0-9-]*\.webp', relative.as_posix())
            unit_index = relative.as_posix() in {'content/README.md', 'content/ART-SPEC.md', 'content/cards/index.json', 'content/enemies/index.json', 'content/bosses/index.json'}
            if not (unit_source or unit_art or unit_index):
                raise RuntimeError('File is not card-game content: '+relative.as_posix())
        if path.suffix.lower() in PRIVATE_SUFFIXES or '.bak' in path.name:
            raise RuntimeError('Private or backup file in a source directory: '+relative.as_posix())
        if path.name.startswith('.env') and path.name != '.env.example':
            raise RuntimeError('Environment file in a source directory: '+relative.as_posix())
        if path.name.endswith(('.old', '.orig', '.tmp', '.tar', '.tgz', '.zip')) or any(part in {'logs', '_codex_backups'} for part in relative.parts):
            raise RuntimeError('Unexpected generated file: '+relative.as_posix())
        files.append(path)
    return files


def load_private_markers(path, files):
    if path is None:
        path = os.environ.get('GACHA_RELEASE_PRIVATE_MARKERS')
        if not path:
            default = ROOT/'.local/release-private-markers.json'
            if not default.exists():
                return ()
            path = default
    try:
        source = Path(path).resolve()
        if source in {file.resolve() for file in files}:
            raise RuntimeError('Private marker configuration must stay outside published files.')
        markers = json.loads(source.read_text(encoding='utf-8'))
    except (OSError, ValueError) as error:
        raise RuntimeError('Cannot read a valid private marker JSON file.') from error
    if not isinstance(markers, list) or not markers or any(
        not isinstance(marker, str) or len(marker.strip()) < 4 or any(ord(character) < 32 for character in marker)
        for marker in markers
    ):
        raise RuntimeError('Private markers must be a non-empty JSON list of strings of at least four characters.')
    return tuple(dict.fromkeys(marker.strip().replace('\\', '/').casefold() for marker in markers))


def scan(files, private_markers=None):
    if private_markers is None:
        private_markers = load_private_markers(None, files)
    needles = {}
    for marker in private_markers:
        variants = {marker, marker.replace('/', '\\'), marker.replace('/', '\\\\')}
        needles[marker] = tuple(variant.encode(encoding).lower() for variant in variants for encoding in ('utf-8', 'utf-16-le', 'utf-16-be'))
    reviews = json.loads((ROOT/'release-review.json').read_text(encoding='utf-8'))['files']
    findings = []
    reviewed_matches = 0
    for path in files:
        content = path.read_bytes()
        relative = path.relative_to(ROOT).as_posix()
        if needles:
            folded = content.lower()
            matched = [marker for marker, values in needles.items() if marker in relative.casefold() or any(value in folded for value in values)]
            if matched:
                display_path = relative
                for marker in matched:
                    display_path = re.sub(re.escape(marker), '[private marker]', display_path, flags=re.IGNORECASE)
                findings.append({'category': 'private_marker', 'path': display_path, 'line': 1})
        try:
            text = content.decode('utf-8')
        except UnicodeDecodeError:
            continue
        review = reviews.get(relative, {})
        approved_hash = review.get('sha256') == hashlib.sha256(content).hexdigest()
        for category, pattern in PATTERNS.items():
            for match in pattern.finditer(text):
                if approved_hash and category in review.get('categories', []):
                    reviewed_matches += 1
                    continue
                findings.append({
                    'category': category, 'path': relative,
                    'line': text.count('\n', 0, match.start())+1,
                })
    return findings, reviewed_matches


def main():
    parser = argparse.ArgumentParser(description='Check source boundaries, credential patterns and private deployment markers.')
    parser.add_argument('--private-markers', help='Private JSON marker list, kept outside the published files.')
    arguments = parser.parse_args()
    files = release_files()
    markers = load_private_markers(arguments.private_markers, files)
    findings, reviewed = scan(files, markers)
    for finding in findings:
        print(f"REVIEW {finding['category']} {finding['path']}:{finding['line']}")
    if findings:
        print('Publication requires manual review; credential values were not printed.', file=sys.stderr)
        return 1
    print(f'Release scan passed: {len(files)} files; {reviewed} previously reviewed fixture/label matches; {len(markers)} private markers checked.')
    return 0


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except (RuntimeError, OSError, ValueError) as error:
        print(str(error), file=sys.stderr)
        raise SystemExit(1)
