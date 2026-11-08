#!/bin/sh
# Run on a trusted offline machine. Output is sensitive; never use inside the repo.
set -eu
umask 077
[ "$#" -eq 1 ] || { echo 'usage: provision.sh /absolute/new-private-directory' >&2; exit 2; }
case "$1" in /*) ;; *) echo 'absolute path required' >&2; exit 2;; esac
parent=$(dirname "$1"); base=$(basename "$1")
[ -d "$parent" ] && [ ! -e "$1" ] || { echo 'parent must exist and target must not exist' >&2; exit 2; }
parent=$(cd "$parent" && pwd -P)
repo=$(cd "$(dirname "$0")/../.." && pwd -P)
case "$parent/$base/" in "$repo/"*) echo 'refusing repository output' >&2; exit 2;; esac
mkdir -m 700 "$parent/$base"
cd "$parent/$base"
mkdir -m 700 ca proxy traefik device
openssl req -x509 -newkey rsa:3072 -noenc -days 3650 -subj /CN=waze-server-ca -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign' -keyout ca/server-ca.key -out ca/server-ca.crt >/dev/null 2>&1
openssl req -x509 -newkey rsa:3072 -noenc -days 3650 -subj /CN=waze-client-ca -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign' -keyout ca/client-ca.key -out ca/client-ca.crt >/dev/null 2>&1
printf 'subjectAltName=DNS:proxy\nextendedKeyUsage=serverAuth\n' > ca/server.ext
printf 'extendedKeyUsage=clientAuth\n' > ca/client.ext
openssl req -new -newkey rsa:3072 -noenc -subj /CN=proxy -keyout proxy/server.key -out ca/server.csr >/dev/null 2>&1
openssl x509 -req -in ca/server.csr -CA ca/server-ca.crt -CAkey ca/server-ca.key -CAcreateserial -days 90 -extfile ca/server.ext -out proxy/server.crt >/dev/null 2>&1
openssl req -new -newkey rsa:3072 -noenc -subj /CN=traefik -keyout traefik/client.key -out ca/client.csr >/dev/null 2>&1
openssl x509 -req -in ca/client.csr -CA ca/client-ca.crt -CAkey ca/client-ca.key -CAcreateserial -days 90 -extfile ca/client.ext -out traefik/client.crt >/dev/null 2>&1
cp ca/client-ca.crt proxy/client-ca.crt
cp ca/server-ca.crt traefik/server-ca.crt
openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n' > device/device_key
printf '\n' >> device/device_key
printf '%s\n' 'Created CA, proxy, traefik and device directories. Keep CA private keys offline; securely copy only the required runtime directories. Set runtime ownership for actual container UIDs; never make keys world-readable.'
