#!/bin/sh
set -eu
export DEBIAN_FRONTEND=noninteractive GOMAXPROCS=1
apt-get update -qq
apt-get install -qy --no-install-recommends ca-certificates curl gcc libc6-dev git mysql-server postgresql python3
service mysql start
service postgresql start
runuser -u postgres -- createuser root
mysql --protocol=socket -e 'CREATE DATABASE gacha_upgrade_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci'
runuser -u postgres -- createdb -O root gacha_upgrade_test
curl -fsSL https://go.dev/dl/?mode=json\&include=all -o /tmp/go-downloads.json
curl -fsSL https://go.dev/dl/go1.25.5.linux-amd64.tar.gz -o /tmp/go.tar.gz
python3 - <<'PY'
import hashlib,json
rows=json.load(open('/tmp/go-downloads.json'))
expected=next(f['sha256'] for r in rows if r['version']=='go1.25.5' for f in r['files'] if f['filename']=='go1.25.5.linux-amd64.tar.gz')
assert hashlib.sha256(open('/tmp/go.tar.gz','rb').read()).hexdigest()==expected
PY
tar -xzf /tmp/go.tar.gz -C /tmp
export PATH=/tmp/go/bin:$PATH
export GACHA_REQUIRE_DB_MATRIX=1
export GACHA_REVIEW_OUTPUT=/out
export TEST_MYSQL_DSN='root@unix(/var/run/mysqld/mysqld.sock)/gacha_authority_test?parseTime=true'
export TEST_POSTGRES_DSN='host=/var/run/postgresql user=root dbname=gacha_authority_test sslmode=disable'
harness=/src/docs/pr-six-unit-guilds/harness
baseline=8877d515dcb59b481709a266d25ea9e156a57213
status=0
for phase in baseline candidate; do
  mysql --protocol=socket -e 'DROP DATABASE IF EXISTS gacha_authority_test; CREATE DATABASE gacha_authority_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci'
  runuser -u postgres -- dropdb --if-exists gacha_authority_test
  runuser -u postgres -- createdb -O root gacha_authority_test
  workspace=$(mktemp -d)
  source=/src
  if [ "$phase" = baseline ]; then
    source=$(mktemp -d)
    git -c safe.directory=/src -C /src archive "$baseline" backend | tar -x -C "$source"
  fi
  cd "$workspace"
  python3 "$harness/setup.py" "$source" "$harness" "/out/$phase-source-hashes.json" "$baseline"
  go mod edit -require=github.com/gin-gonic/gin@v1.10.1 -require=github.com/glebarez/sqlite@v1.11.0 -require=gorm.io/driver/mysql@v1.6.0 -require=gorm.io/driver/postgres@v1.6.0
  # Baseline's two documented stale test fixtures are checked by summarize.py.
  # Production source and original baseline assertions remain unchanged.
  REVIEW_PHASE="$phase" go test -mod=mod -p 1 -parallel 1 -count=1 -timeout=12m -json ./model ./controller -skip '^TestReview' > "/out/$phase-tests.jsonl" 2> "/out/$phase-build.txt" || { if [ "$phase" = candidate ]; then status=1; fi; }
  if [ "$phase" = baseline ]; then case_name=TestReviewProduceLegacy; else case_name=TestReviewUpgradeLegacy; fi
  REVIEW_PHASE="$phase" go test -mod=mod -p 1 -parallel 1 -count=1 -timeout=8m -json ./model -run "^$case_name$" > "/out/$phase-upgrade-tests.jsonl" 2> "/out/$phase-upgrade-build.txt" || status=1
  if [ "$phase" = candidate ]; then
    go test -mod=mod -p 1 -parallel 1 -count=1 -timeout=8m -json ./model -run '^TestReviewActualGuildCombatCorpus$' > /out/actual-combat-tests.jsonl 2> /out/actual-combat-build.txt || status=1
  fi
  if [ -s "/out/$phase-build.txt" ]; then cat "/out/$phase-build.txt"; fi
done
python3 "$harness/summarize.py"
exit "$status"
