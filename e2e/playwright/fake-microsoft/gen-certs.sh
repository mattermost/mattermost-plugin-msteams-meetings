#!/bin/sh
# Creates a throwaway CA and a certificate for the Microsoft hosts the fake answers for,
# then a CA bundle (system roots plus the throwaway CA) for Mattermost's SSL_CERT_FILE.
set -eu

OUT=/certs
cd "$OUT"

if [ -f fake.crt ] && [ -f ca-bundle.crt ]; then
  exit 0
fi

openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -keyout ca.key -out ca.crt -subj "/CN=Fake Microsoft E2E CA"

openssl req -newkey rsa:2048 -nodes -keyout fake.key -out fake.csr -subj "/CN=login.microsoftonline.com"
printf "subjectAltName=DNS:login.microsoftonline.com,DNS:graph.microsoft.com\n" > san.ext
openssl x509 -req -in fake.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 3650 -extfile san.ext -out fake.crt

cat /etc/ssl/certs/ca-certificates.crt ca.crt > ca-bundle.crt
chmod 644 ./*
