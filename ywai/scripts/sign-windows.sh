#!/usr/bin/env bash
# Authenticode-sign one Windows binary, in place.
#
# Windows Defender's ML classifier flags unsigned Go binaries — the usual
# verdict is Trojan:Win32/Bearfoos.A!ml — and Smart App Control refuses to run
# them at all ("malicious binary reputation"). Neither is about what the code
# does: every release ships a new hash with no reputation behind it, and a
# signature is what carries reputation across releases.
#
# No-op unless the certificate is configured, so a fork or a release without
# the secrets still publishes instead of failing the pipeline. goreleaser has
# no `disable` on binary_signs, which is why this lives in a script.
set -euo pipefail

artifact="${1:?usage: sign-windows.sh <artifact>}"

case "$artifact" in
    *.exe) ;;
    *) exit 0 ;;  # only Windows binaries are Authenticode-signed
esac

if [ -z "${YWAI_WINDOWS_CERT:-}" ]; then
    echo "sign-windows: no certificate configured, leaving $(basename "$artifact") unsigned"
    exit 0
fi
if [ ! -f "$YWAI_WINDOWS_CERT" ]; then
    echo "sign-windows: YWAI_WINDOWS_CERT points at a missing file: $YWAI_WINDOWS_CERT" >&2
    exit 1
fi

tmp="${artifact}.signed"
osslsigncode sign \
    -pkcs12 "$YWAI_WINDOWS_CERT" \
    -pass "${YWAI_WINDOWS_CERT_PASSWORD:-}" \
    -n "ywai" \
    -i "https://github.com/YoizenSA/dev-ai-workflow" \
    -t "http://timestamp.digicert.com" \
    -in "$artifact" \
    -out "$tmp"
mv -f "$tmp" "$artifact"
echo "sign-windows: signed $(basename "$artifact")"
