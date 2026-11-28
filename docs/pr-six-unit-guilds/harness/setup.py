"""Copy game sources unchanged into a disposable host-contract test module."""
from pathlib import Path
import hashlib,json,shutil,subprocess,sys

source,harness,out=map(Path,sys.argv[1:4])
baseline=sys.argv[4]
repository=harness.resolve().parents[2]
for directory in ('model','controller','common','middleware'):Path(directory).mkdir()
manifest={}
for part in ('model','controller'):
    for p in sorted((source/'backend'/part).iterdir()):
        if p.suffix not in ('.go','.json'):continue
        shutil.copy2(p,Path(part)/p.name)
        manifest[str(p.relative_to(source))]=hashlib.sha256(p.read_bytes()).hexdigest()
shutil.copy2(source/'backend/integration/gacha-database.go.example','model/bootstrap.go')
shutil.copy2(source/'backend/integration/gacha-ladder-worker.go.example','model/review_worker.go')
for name in ('adapter','common'):
    shutil.copy2(harness/(name+'.go.example'),('model' if name=='adapter' else 'common')+'/'+name+'.go')
for fixture,target in [('docs/pr-2-evidence/harness/session.go.example','middleware/session.go'),('docs/pr-3-evidence/harness/go.mod.example','go.mod')]:
    data=subprocess.check_output(['git','-c',f'safe.directory={repository}','-C',str(repository),'show',f'{baseline}:{fixture}'])
    Path(target).write_bytes(data)
shutil.copy2(harness/'upgrade_test.go.example','model/review_upgrade_test.go')
if (source/'backend/model/gacha_guild_progression.go').exists():
    shutil.copy2(harness/'balance_test.go.example','model/review_balance_test.go')
out.write_text(json.dumps(manifest,indent=2)+'\n')
