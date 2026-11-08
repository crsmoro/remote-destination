#!/usr/bin/env bash
# Stage one leaf only; this script never activates, reloads, or restarts services.
set -Eeuo pipefail
umask 077
if [ "$#" -ne 7 ]; then
  echo 'usage: stage-mtls-leaf.sh server|client CA_CERT CA_KEY EXISTING_LEAF_KEY NEW_OUTPUT_DIR DNS_OR_CLIENT_CN DAYS' >&2
  exit 2
fi
role=$1 ca=$2 cakey=$3 leafkey=$4 out=$5 identity=$6 days=$7
[[ $role == server || $role == client ]] || { echo 'invalid role' >&2; exit 2; }
[[ $identity =~ ^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$ && $identity != *..* ]] || { echo 'invalid identity' >&2; exit 2; }
[[ $days =~ ^[1-9][0-9]{0,2}$ ]] && (( days <= 365 )) || { echo 'days must be 1..365' >&2; exit 2; }
[[ ! -e $out && ! -L $out ]] || { echo 'output already exists' >&2; exit 2; }
parent=$(dirname -- "$out")
[[ -d $parent && ! -L $parent ]] || { echo 'output parent missing or symlink' >&2; exit 2; }
tmp=$(mktemp -d "$parent/.mtls-stage.XXXXXXXX")
cleanup() { [[ -z ${tmp:-} ]] || rm -rf -- "$tmp"; }
trap cleanup EXIT
fail() { echo "$1" >&2; exit 1; }
openssl x509 -in "$ca" -noout -checkend "$(( (days + 1) * 86400 ))" >/dev/null 2>&1 || fail 'CA expires too soon or is invalid'
openssl x509 -in "$ca" -pubkey -noout >"$tmp/ca.pub" 2>/dev/null || fail 'invalid CA certificate'
openssl pkey -in "$cakey" -pubout >"$tmp/ca-key.pub" 2>/dev/null || fail 'invalid CA key'
cmp -s "$tmp/ca.pub" "$tmp/ca-key.pub" || fail 'CA certificate and key mismatch'
openssl pkey -in "$leafkey" -check -noout >/dev/null 2>&1 || fail 'invalid leaf key'
cp -- "$leafkey" "$tmp/leaf.key"
chmod 600 "$tmp/leaf.key"
openssl req -new -key "$tmp/leaf.key" -subj "/CN=$identity" -out "$tmp/leaf.csr" >/dev/null 2>&1 || fail 'CSR failed'
if [[ $role == server ]]; then
  printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:%s\nsubjectKeyIdentifier=hash\nauthorityKeyIdentifier=keyid,issuer\n' "$identity" >"$tmp/ext"
  purpose=sslserver
else
  printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\nsubjectKeyIdentifier=hash\nauthorityKeyIdentifier=keyid,issuer\n' >"$tmp/ext"
  purpose=sslclient
fi
# A random 128-bit positive serial avoids shared serial-file state and concurrent collisions.
serial=$(openssl rand -hex 16)
openssl x509 -req -in "$tmp/leaf.csr" -CA "$ca" -CAkey "$cakey" -set_serial "0x$serial" -days "$days" -sha256 -extfile "$tmp/ext" -out "$tmp/leaf.crt" >/dev/null 2>&1 || fail 'signing failed'
verify=(-x509_strict -purpose "$purpose" -CAfile "$ca")
[[ $role != server ]] || verify+=(-verify_hostname "$identity")
openssl verify "${verify[@]}" "$tmp/leaf.crt" >/dev/null 2>&1 || fail 'strict leaf verification failed'
openssl x509 -in "$tmp/leaf.crt" -pubkey -noout >"$tmp/cert.pub" 2>/dev/null
openssl pkey -in "$tmp/leaf.key" -pubout >"$tmp/key.pub" 2>/dev/null
cmp -s "$tmp/cert.pub" "$tmp/key.pub" || fail 'leaf key mismatch'
rm -f -- "$tmp/leaf.csr" "$tmp/ext" "$tmp/ca.pub" "$tmp/ca-key.pub" "$tmp/cert.pub" "$tmp/key.pub"
# Refuse collisions, including concurrent stages. Never replace an existing output.
mkdir -- "$out" 2>/dev/null || fail 'output already exists or cannot be created'
if ! mv -- "$tmp/leaf.crt" "$tmp/leaf.key" "$out/"; then rmdir -- "$out" 2>/dev/null || true; fail 'stage move failed'; fi
printf 'Staged %s leaf in %s; NOT activated.\n' "$role" "$out"
