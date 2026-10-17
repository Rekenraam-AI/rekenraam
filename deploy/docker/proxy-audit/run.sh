#!/usr/bin/env bash
# Runs the #268 reverse-proxy audit end to end against a disposable deployment:
# the supplied Compose file plus a Caddy TLS proxy with a local CA. Everything
# it creates (volumes, keys, CA, cookie jars) is synthetic and is removed again
# unless KEEP=1. See README.md for what each phase shows.
#
#   deploy/docker/proxy-audit/run.sh
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
work="${PROXY_AUDIT_WORKDIR:-$(mktemp -d)}"
umask 077
project=rr-proxy-audit
url=https://rekenraam.test:8443

compose() {
	docker compose -p "$project" --env-file "$work/audit.env" \
		-f "$repo/deploy/docker/compose.yaml" -f "$repo/deploy/docker/proxy-audit/compose.yaml" "$@"
}
cleanup() {
	if [[ "${KEEP:-0}" != 1 ]]; then
		compose down -v >/dev/null 2>&1 || true
		rm -rf "$work"
	fi
}
trap cleanup EXIT

set_key() { sed -i "s#^REKENRAAM_SECRET_KEY=.*#REKENRAAM_SECRET_KEY=$1#" "$work/audit.env"; }
https() { curl -sS --resolve rekenraam.test:8443:127.0.0.1 --cacert "$work/ca.crt" "$@"; }
playwright() {
	(cd "$repo/e2e" && set -a && . "$work/audit.env" && set +a &&
		PROXY_AUDIT_STATE="$work/state.json" PROXY_AUDIT_PHASE="$1" \
			./node_modules/.bin/playwright test -c proxy-audit.config.ts --reporter=list)
}
wait_healthy() {
	for _ in $(seq 1 30); do
		https -o /dev/null -f "$url/healthz" 2>/dev/null && return 0
		sleep 1
	done
	echo "deployment did not become healthy" >&2
	return 1
}
csrf() { https -b "$work/jar" "$url/api/v1/auth/session" | node -e 'process.stdin.on("data",d=>console.log(JSON.parse(d).csrf_token))'; }
totp() {
	local code
	until code="$(cd "$repo/e2e" && PROXY_AUDIT_STATE="$work/state.json" node proxy-audit/totp-code.ts)"; do sleep 5; done
	echo "$code"
}

printf 'SETUP_TOKEN=%s\nREKENRAAM_SECRET_KEY=%s\n' "$(openssl rand -base64 32)" "$(openssl rand -base64 32)" >"$work/audit.env"
echo "== fresh deployment (workdir $work)"
compose down -v >/dev/null 2>&1 || true
compose up -d --build --quiet-pull
until docker cp "${project}-proxy-1:/data/caddy/pki/authorities/local/root.crt" "$work/ca.crt" 2>/dev/null; do sleep 1; done
wait_healthy

echo "== TLS chain verified against the local CA; headers through the proxy"
https -D - -o /dev/null "$url/" | grep -iE '^(HTTP|strict-transport|content-security|x-frame|cache-control|referrer)'
https -D - -o /dev/null "$url/api/v1/setup/status" | grep -iE '^(cache-control|pragma)'

echo "== browser: onboarding, cookies, MFA, tabs, history, caches"
playwright deploy

echo "== spoofed client headers through the proxy (expect 401, recorded IP = proxy-observed peer)"
body='{"username":"probe-unknown","password":"x"}'
https -o /dev/null -w '%{http_code}\n' -X POST "$url/api/v1/auth/login" -H "Origin: $url" -H 'Content-Type: application/json' \
	-H 'X-Forwarded-For: 203.0.113.66' -H 'Forwarded: for=203.0.113.67;proto=https;host=evil.example' \
	-H 'X-Forwarded-Host: evil.example' -H 'X-Forwarded-Proto: http' -H 'X-Real-IP: 203.0.113.68' -d "$body"
echo "== foreign origins (expect 403 each)"
for origin in https://evil.example http://rekenraam.test:8443; do
	https -o /dev/null -w "$origin %{http_code}\n" -X POST "$url/api/v1/auth/login" -H "Origin: $origin" \
		-H 'X-Forwarded-Host: evil.example' -H 'Content-Type: application/json' -d "$body"
done
echo "== unknown Host through the proxy (expect an empty Caddy response, no app headers)"
https -D - -o /dev/null "$url/api/v1/setup/status" -H 'Host: evil.example' | grep -iE '^(HTTP|x-request-id|content-length)'
echo "== direct to the loopback app port with forged proxy headers (expect 403, no HSTS, no cookie)"
curl -sS -D - -o /dev/null -X POST http://127.0.0.1:16888/api/v1/auth/login -H "Origin: $url" \
	-H 'X-Forwarded-Host: rekenraam.test:8443' -H 'X-Forwarded-Proto: https' -H 'X-Forwarded-For: 203.0.113.69' \
	-H 'Content-Type: application/json' -d "$body" | grep -iE '^(HTTP|strict-transport|set-cookie)' || true

echo "== recorded client addresses (none may be 203.0.113.x)"
https -c "$work/jar" -o /dev/null -X POST "$url/api/v1/auth/login" -H "Origin: $url" -H 'Content-Type: application/json' \
	-d '{"username":"owner","password":"proxy-audit-password"}'
https -b "$work/jar" -c "$work/jar" -o /dev/null -w 'mfa sign-in %{http_code}\n' -X POST "$url/api/v1/auth/login/mfa" \
	-H "Origin: $url" -H 'Content-Type: application/json' -d "{\"code\":\"$(totp)\"}"
https -b "$work/jar" "$url/api/v1/auth/events?limit=50" |
	node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const e=JSON.parse(s).events;const ips=[...new Set(e.map(x=>x.client_ip))];console.log("client_ip values:",ips.join(", "));if(ips.some(ip=>ip.startsWith("203.0.113.")))process.exit(1)})'

echo "== restart with the same key"
compose restart rekenraam >/dev/null
wait_healthy
https -b "$work/jar" -o /dev/null -w 'existing session after restart %{http_code}\n' "$url/api/v1/accounts"
playwright restart

echo "== restart with a rotated key"
set_key "$(openssl rand -base64 32)"
compose up -d rekenraam >/dev/null
wait_healthy
playwright key-changed

echo "== restart with no key"
set_key ""
compose up -d rekenraam >/dev/null
wait_healthy
https -b "$work/jar" "$url/api/v1/auth/mfa"; echo
playwright key-changed
compose logs rekenraam 2>&1 | grep -c 'mfa secret could not be opened' | sed 's/^/operator warnings logged: /'

echo "== audit run complete"
