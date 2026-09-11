#!/usr/bin/env bash
# Dev helper: shim Windows docker.exe as `docker` for WSL bash, then run the
# docker matrix with it. Lives outside the matrix scripts on purpose.
set -e
shim_dir=/mnt/c/Users/Nahuel/AppData/Local/Temp/opencode/shim
mkdir -p "$shim_dir"
ln -sf "$(command -v docker.exe)" "$shim_dir/docker"
export PATH="$shim_dir:$PATH"
# docker.exe needs a Windows-visible path for -f; WSL paths do not translate.
export YWAI_COMPOSE_FILE="$(wslpath -w "$(dirname "$0")/compose.yaml")"
exec bash "$(dirname "$0")/run-matrix.sh" "$@"
