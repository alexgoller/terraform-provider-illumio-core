#!/usr/bin/env bash
#
# Stepwise acceptance suite: create one resource at a time, check it against the
# PCE, then tear it down in reverse.
#
# Every step is verified by reading the object back from the API, so a step that
# reports OK has been confirmed on the PCE, not just in Terraform state.
#
# Usage:
#   ENV_FILE=../.env-25.2 ./suite.sh list
#   ENV_FILE=../.env-25.2 ./suite.sh create 1      # one step
#   ENV_FILE=../.env-25.2 ./suite.sh create 1-5    # a range
#   ENV_FILE=../.env-25.2 ./suite.sh verify 10     # re-read from the PCE
#   ENV_FILE=../.env-25.2 ./suite.sh status        # what exists so far
#   ENV_FILE=../.env-25.2 ./suite.sh destroy 17    # one step, reverse order
#   ENV_FILE=../.env-25.2 ./suite.sh teardown      # the full two-pass teardown
#
# The env file supplies PCE_HOST, PCE_PORT, PCE_ORG_ID, API_KEY, API_SECRET.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${ENV_FILE:-${HERE}/../.env}"

# addr:label  — order matters, it is the dependency order
STEPS=(
  "illumio-core_label.app:label app"
  "illumio-core_label.env:label env"
  "illumio-core_label.role_web:label role=web"
  "illumio-core_label.role_db:label role=db"
  "illumio-core_label_group.roles:label group"
  "illumio-core_ip_list.corporate:ip list (corporate)"
  "illumio-core_ip_list.untrusted:ip list (untrusted, with fqdn)"
  "illumio-core_service.https:service https"
  "illumio-core_service.ssh:service ssh"
  "illumio-core_rule_set.rs:rule set with AND-ed app scope"
  "illumio-core_security_rule.ringfence:security rule (ring-fence, ams to ams)"
  "illumio-core_security_rule.web_to_db:security rule (web to db)"
  "illumio-core_deny_rule.no_ssh:deny rule"
  "illumio-core_deny_rule.override_ssh:override-deny rule"
  "illumio-core_enforcement_boundary.eb:enforcement boundary"
  "illumio-core_unmanaged_workload.wl:unmanaged workload"
  "illumio-core_provisioning.policy:provisioning (explicit hrefs)"
)

load_env() {
  [ -f "$ENV_FILE" ] || { echo "env file not found: $ENV_FILE" >&2; exit 1; }
  set -a; . "$ENV_FILE"; set +a
  export ILLUMIO_PCE_HOST="https://${PCE_HOST}:${PCE_PORT}"
  export ILLUMIO_PCE_ORG_ID="${PCE_ORG_ID}"
  export ILLUMIO_API_KEY_USERNAME="${API_KEY}"
  export ILLUMIO_API_KEY_SECRET="${API_SECRET}"
  API="https://${PCE_HOST}:${PCE_PORT}/api/v2"
}

pce() { curl -sS --max-time 60 -u "${API_KEY}:${API_SECRET}" "$@"; }

addr_of() { echo "${STEPS[$1-1]%%:*}"; }
desc_of() { echo "${STEPS[$1-1]#*:}"; }

# Read the object back from the PCE and show the fields that matter.
verify_step() {
  local n="$1" addr href
  addr="$(addr_of "$n")"
  href="$(terraform show -json 2>/dev/null | python3 -c "
import json,sys
try: d=json.load(sys.stdin)
except Exception: sys.exit()
for r in d.get('values',{}).get('root_module',{}).get('resources',[]):
    if r['address']=='${addr}':
        print(r['values'].get('href') or '')
        break" || true)"

  if [ -z "$href" ]; then
    echo "    not in state"
    return 1
  fi

  if [ "$addr" = "illumio-core_provisioning.policy" ]; then
    terraform show -json | python3 -c "
import json,sys
d=json.load(sys.stdin)
for r in d['values']['root_module']['resources']:
    if r['address']=='${addr}':
        v=r['values']
        print(f\"    policy version {v.get('version')}  ({v.get('version_href')})\")
        print(f\"    pending after provisioning: {v.get('pending_hrefs')}\")"
    return 0
  fi

  pce "${API}${href}" | python3 -c "
import json,sys
d=json.load(sys.stdin)
print(f\"    href     {d.get('href')}\")
for k in ('name','key','value','hostname','description','enabled','override'):
    if d.get(k) not in (None,''): print(f\"    {k:8s} {d[k]!r}\")
for k in ('scopes','providers','consumers','ingress_services','ip_ranges','fqdns','service_ports','labels','sub_groups'):
    if d.get(k): print(f\"    {k:8s} {len(d[k])} entr{'y' if len(d[k])==1 else 'ies'}\")"
}

cmd_list() {
  local i=1
  for s in "${STEPS[@]}"; do
    printf "  %2d  %-44s %s\n" "$i" "${s##*:}" "${s%%:*}"
    i=$((i+1))
  done
}

expand_range() {
  case "$1" in
    *-*) seq "${1%-*}" "${1#*-}" ;;
    *)   echo "$1" ;;
  esac
}

cmd_create() {
  for n in $(expand_range "$1"); do
    echo "── step ${n}: $(desc_of "$n")"
    terraform apply -auto-approve -no-color -target="$(addr_of "$n")" >/tmp/suite.log 2>&1 || {
      echo "    APPLY FAILED"; grep -E "Error" /tmp/suite.log | head -3 | sed 's/^/    /'; return 1; }
    echo "    created — reading back from the PCE:"
    verify_step "$n"
  done
}

cmd_destroy() {
  for n in $(expand_range "$1"); do
    echo "── destroy step ${n}: $(desc_of "$n")"
    terraform destroy -auto-approve -no-color -target="$(addr_of "$n")" >/tmp/suite.log 2>&1 || {
      echo "    DESTROY FAILED"; grep -E "Error" /tmp/suite.log | head -3 | sed 's/^/    /'; return 1; }
    echo "    destroyed"
  done
}

cmd_status() {
  local i=1
  for s in "${STEPS[@]}"; do
    if terraform state list 2>/dev/null | grep -qx "${s%%:*}"; then
      printf "  %2d  [x] %s\n" "$i" "${s##*:}"
    else
      printf "  %2d  [ ] %s\n" "$i" "${s##*:}"
    fi
    i=$((i+1))
  done
}

# Destroy is two-pass on provisioned policy: objects the active policy still
# references cannot be deleted until their deletion is provisioned.
cmd_teardown() {
  echo "── capturing hrefs before anything leaves state"
  terraform show -json | python3 -c "
import json,sys
d=json.load(sys.stdin)
want={'illumio-core_rule_set','illumio-core_ip_list','illumio-core_service',
      'illumio-core_label_group','illumio-core_enforcement_boundary'}
hrefs=[r['values']['href'] for r in d['values']['root_module']['resources'] if r['type'] in want]
json.dump(hrefs, open('/tmp/suite_hrefs.json','w'))
print(f'    {len(hrefs)} provisionable objects recorded')"

  echo "── pass 1: terraform destroy"
  terraform destroy -auto-approve -no-color >/tmp/suite.log 2>&1 && echo "    all resources destroyed" || {
    echo "    partial — objects the active policy still references were refused:"
    grep -oE '"token":"[a-z_]+"' /tmp/suite.log | sort -u | sed 's/^/      /'
  }

  echo "── pass 2: provision the pending deletions"
  python3 - <<'PY' > /tmp/suite_cs.json
import json
cs={}
for h in json.load(open('/tmp/suite_hrefs.json')):
    cs.setdefault(h.split('/sec_policy/')[1].split('/')[1], []).append({"href": h})
print(json.dumps({"update_description": "tf-suite teardown", "change_subset": cs}))
PY
  pce -X POST -H "Content-Type: application/json" --data-binary @/tmp/suite_cs.json \
      -o /tmp/suite_prov.json -w "    POST /sec_policy -> %{http_code}\n" \
      "${API}/orgs/${PCE_ORG_ID}/sec_policy"

  echo "── pass 3: terraform destroy again"
  terraform destroy -auto-approve -no-color >/tmp/suite.log 2>&1 && echo "    complete" || {
    echo "    still failing:"; grep -E "Error" /tmp/suite.log | head -3 | sed 's/^/      /'; }

  echo "── leftovers"
  cmd_leftovers
}

cmd_leftovers() {
  for ep in labels sec_policy/draft/label_groups sec_policy/draft/ip_lists \
            sec_policy/draft/services sec_policy/draft/rule_sets \
            sec_policy/draft/enforcement_boundaries workloads; do
    n=$(pce "${API}/orgs/${PCE_ORG_ID}/${ep}?max_results=1000" | python3 -c "
import json,sys
print(sum(1 for o in json.load(sys.stdin)
          if 'tf-suite' in str(o.get('name',''))+str(o.get('value',''))+str(o.get('hostname',''))))" 2>/dev/null || echo "?")
    [ "$n" != "0" ] && printf "    %-40s %s\n" "$ep" "$n"
  done
  echo "    (nothing listed above means the PCE is clean)"
}

load_env
cd "$HERE"

case "${1:-list}" in
  list)      cmd_list ;;
  create)    cmd_create "${2:?step or range}" ;;
  verify)    echo "── step ${2}: $(desc_of "${2}")"; verify_step "${2}" ;;
  destroy)   cmd_destroy "${2:?step or range}" ;;
  status)    cmd_status ;;
  teardown)  cmd_teardown ;;
  leftovers) cmd_leftovers ;;
  *)         sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' ;;
esac
