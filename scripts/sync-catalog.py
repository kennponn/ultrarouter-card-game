"""Compile card-owned data into the browser display catalog and Go embedded catalog."""
from pathlib import Path
import argparse
import json
import re
import sys


ROOT = Path(__file__).resolve().parents[1]
GROUPS = {'cards': 'card.json', 'enemies': 'enemy.json', 'bosses': 'boss.json'}


def read_json(path):
    return json.loads(path.read_text(encoding='utf-8'), parse_constant=lambda value: fail('Non-finite JSON number: '+value))


def fail(message):
    raise ValueError(message)


def encode(value):
    return json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False)


def load_units(root=ROOT):
    groups, aliases, all_ids = {}, {}, set()
    for group, filename in GROUPS.items():
        index = read_json(root/'content'/group/'index.json')
        ids = index.get('ids')
        if index.get('schemaVersion') != 1 or not isinstance(ids, list) or not ids:
            fail('Invalid unit index: '+group)
        units = []
        for identity in ids:
            if not isinstance(identity, str) or not re.fullmatch(r'[a-z][a-z0-9-]*', identity) or identity in all_ids:
                fail('Invalid or duplicate unit ID in '+group)
            all_ids.add(identity)
            directory = root/'content'/group/identity
            unit = read_json(directory/filename)
            if unit.get('id') != identity or not unit.get('name'):
                fail('Unit ID/name does not match its directory: '+identity)
            for field in ('img', 'battleImg'):
                image = unit.get(field, '')
                path = (root/image).resolve()
                if not image or not path.is_relative_to((directory/'assets').resolve()) or path.suffix != '.webp' or not path.is_file():
                    fail('Missing or unscoped '+field+' for '+identity)
            for old, target in unit.pop('legacyImages', {}).items():
                if old in aliases or not old.startswith('assets/') or '..' in Path(old).parts or not (root/target).resolve().is_relative_to((directory/'assets').resolve()) or not (root/target).is_file():
                    fail('Invalid legacy image mapping for '+identity)
                aliases[old] = target
            if group == 'cards':
                if unit.get('rarity') not in {'R', 'SR', 'SSR'} or not all(name in unit.get('skills', {}) for name in ('normal', 'skill')):
                    fail('Invalid rarity or skills for '+identity)
            elif unit.get('boss') != (group == 'bosses'):
                fail('Enemy/boss folder classification mismatch: '+identity)
            units.append(unit)
        found = {path.parent.name for path in (root/'content'/group).glob('*/'+filename)}
        if found != set(ids):
            fail('Unit index must include every '+filename+' in '+group)
        groups[group] = units
    return groups, aliases


def generated_block(name, value):
    return f'// BEGIN GENERATED {name} — run python scripts/sync-catalog.py\nconst {name} = {encode(value)};\n// END GENERATED {name}'


def replace_block(source, name, value):
    pattern = rf'// BEGIN GENERATED {name}[^\n]*\n.*?// END GENERATED {name}'
    block = generated_block(name, value)
    if re.search(pattern, source, re.S):
        return re.sub(pattern, lambda match: block, source, count=1, flags=re.S)
    pattern = rf'^const {name} = \[.*?^\];'
    result, count = re.subn(pattern, lambda match: block, source, count=1, flags=re.S | re.M)
    if count != 1:
        fail('Cannot locate generated catalog section: '+name)
    return result


def outputs(root=ROOT):
    groups, aliases = load_units(root)
    girls = groups['cards']
    # Preserve the historical normal-enemy-then-boss order consumed by host code.
    enemies = groups['enemies'] + groups['bosses']
    catalog_path = root/'backend/model/gacha_catalog.json'
    catalog = read_json(catalog_path)
    catalog['GIRLS'], catalog['ENEMY_TPL'] = girls, enemies
    source_path = root/'js/data.js'
    source = source_path.read_text(encoding='utf-8')
    source = replace_block(source, 'GIRLS', girls)
    source = replace_block(source, 'ENEMY_TPL', enemies)
    alias_block = generated_block('UNIT_ASSET_ALIASES', aliases)
    alias_pattern = r'// BEGIN GENERATED UNIT_ASSET_ALIASES[^\n]*\n.*?// END GENERATED UNIT_ASSET_ALIASES'
    if re.search(alias_pattern, source, re.S):
        source = re.sub(alias_pattern, lambda match: alias_block, source, count=1, flags=re.S)
    else:
        source = source.replace('// END GENERATED ENEMY_TPL', '// END GENERATED ENEMY_TPL\n\n'+alias_block+
            '\n\n// Resolve persisted battle-report image paths after the card folder migration.\n'
            'function unitAsset(src) { return UNIT_ASSET_ALIASES[src] || src; }')
    return {catalog_path: encode(catalog)+'\n', source_path: source}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Fail if generated catalogs need updating; do not write files.')
    args = parser.parse_args()
    changed = []
    for path, content in outputs().items():
        if path.read_text(encoding='utf-8') != content:
            changed.append(path.relative_to(ROOT).as_posix())
            if not args.check:
                newline = '\r\n' if b'\r\n' in path.read_bytes() else '\n'
                path.write_text(content, encoding='utf-8', newline=newline)
    if args.check and changed:
        print('Catalog is out of date: '+', '.join(changed), file=sys.stderr)
        return 1
    print('Unit catalogs verified.' if args.check else 'Unit catalogs synchronized: '+(', '.join(changed) or 'already current'))
    return 0


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(str(error), file=sys.stderr)
        raise SystemExit(1)
